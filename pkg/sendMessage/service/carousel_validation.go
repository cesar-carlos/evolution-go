package send_service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateCarousel checks every card before connecting or fetching media.
func ValidateCarousel(data *CarouselStruct) error {
	if data == nil {
		return errors.New("message is required")
	}
	if strings.TrimSpace(data.Number) == "" {
		return errors.New("phone number is required")
	}
	if len(data.Cards) == 0 {
		return errors.New("at least one card is required")
	}
	for i, card := range data.Cards {
		if strings.TrimSpace(card.Body.Text) == "" {
			return fmt.Errorf("card %d body text is required", i+1)
		}
		if card.Header.ImageUrl != "" && card.Header.VideoUrl != "" {
			return fmt.Errorf("card %d must use either imageUrl or videoUrl", i+1)
		}
		for _, raw := range []string{card.Header.ImageUrl, card.Header.VideoUrl} {
			if raw != "" && !validCarouselURL(raw) {
				return fmt.Errorf("card %d media URL must be absolute HTTP(S)", i+1)
			}
		}
		reply, cta := false, false
		for j, button := range card.Buttons {
			name, _, err := carouselButtonParams(button)
			if err != nil {
				return fmt.Errorf("card %d button %d: %w", i+1, j+1, err)
			}
			if name == "quick_reply" {
				reply = true
			} else {
				cta = true
			}
		}
		if reply && cta {
			return fmt.Errorf("card %d cannot mix REPLY and CTA buttons", i+1)
		}
	}
	return nil
}

func validCarouselURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}

func carouselButtonParams(btn CarouselButtonStruct) (string, map[string]string, error) {
	if strings.TrimSpace(btn.DisplayText) == "" {
		return "", nil, errors.New("displayText is required")
	}
	kind := strings.ToUpper(btn.Type)
	if kind == "" {
		kind = "REPLY"
	}
	params := map[string]string{"display_text": btn.DisplayText}
	target := btn.Id
	var name, key string
	switch kind {
	case "URL", "CALL":
		alias := btn.URL
		name, key = "cta_url", "url"
		if kind == "CALL" {
			alias = btn.PhoneNumber
			name, key = "cta_call", "phone_number"
		}
		if alias != "" {
			if target != "" && target != alias {
				return "", nil, errors.New("id conflicts with button target")
			}
			target = alias
		}
		if kind == "URL" && !validCarouselURL(target) {
			return "", nil, errors.New("button URL must be absolute HTTP(S)")
		}
	case "COPY", "COPY_CODE":
		name, key, target = "cta_copy", "copy_code", btn.CopyCode
	case "REPLY":
		name, key = "quick_reply", "id"
	default:
		return "", nil, errors.New("unsupported carousel button type")
	}
	if strings.TrimSpace(target) == "" {
		return "", nil, errors.New("button target is required")
	}
	params[key] = target
	return name, params, nil
}
