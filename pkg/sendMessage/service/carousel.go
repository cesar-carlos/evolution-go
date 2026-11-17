package send_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"time"
)

// SendCarousel prepares media with the request context, then uses the shared send pipeline.
func (s *sendService) SendCarousel(ctx context.Context, data *CarouselStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	if err := ValidateCarousel(data); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, errors.New("instance is required")
	}
	client, err := s.ensureSendClient(ctx, instance.Id)
	if err != nil {
		return nil, err
	}
	msg, err := buildCarouselMessage(ctx, data, client)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.sendMessageContext(ctx, instance, msg, "InteractiveMessage", &SendDataStruct{
		Number: data.Number, Delay: data.Delay, FormatJid: data.FormatJid, Quoted: data.Quoted,
	})
}
func buildCarouselMessage(ctx context.Context, data *CarouselStruct, uploader interactiveMediaUploader) (*waE2E.Message, error) {
	if err := ValidateCarousel(data); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Build carousel cards
	cards := make([]*waE2E.InteractiveMessage, len(data.Cards))
	messageVersion := int32(1)

	for i, card := range data.Cards {
		// Each card MUST have both header and body for carousel to work
		interactiveCard := &waE2E.InteractiveMessage{
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String(card.Body.Text),
			},
			Header: &waE2E.InteractiveMessage_Header{
				Title:              proto.String(card.Header.Title),
				Subtitle:           proto.String(card.Header.Subtitle),
				HasMediaAttachment: proto.Bool(false),
			},
		}

		if card.Header.ImageUrl != "" || card.Header.VideoUrl != "" {
			header, err := prepareInteractiveHeader(ctx, uploader, card.Header.ImageUrl, card.Header.VideoUrl)
			if err != nil {
				return nil, fmt.Errorf("card %d media: %w", i+1, err)
			}
			header.Title = proto.String(card.Header.Title)
			header.Subtitle = proto.String(card.Header.Subtitle)
			interactiveCard.Header = header
		}

		// Add footer if exists
		if card.Footer != "" {
			interactiveCard.Footer = &waE2E.InteractiveMessage_Footer{
				Text: proto.String(card.Footer),
			}
		}

		// Add buttons if exist
		if len(card.Buttons) > 0 {
			buttons := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, len(card.Buttons))
			for j, btn := range card.Buttons {
				buttonName, buttonParamsMap, err := carouselButtonParams(btn)
				if err != nil {
					return nil, err
				}

				encodedParams, err := json.Marshal(buttonParamsMap)
				if err != nil {
					return nil, fmt.Errorf("carousel button params: %w", err)
				}
				buttonParams := string(encodedParams)

				buttons[j] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					Name:             proto.String(buttonName),
					ButtonParamsJSON: proto.String(buttonParams),
				}
			}

			// Cards in carousel: do NOT set MessageParamsJSON or MessageVersion
			// (matching PAPI Node.js behavior for iOS compatibility)
			interactiveCard.InteractiveMessage = &waE2E.InteractiveMessage_NativeFlowMessage_{
				NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
					Buttons: buttons,
				},
			}
		}

		cards[i] = interactiveCard
	}

	// Build carousel message (do NOT set CarouselCardType - matching PAPI Node.js for iOS)
	interactiveMsg := &waE2E.InteractiveMessage{
		InteractiveMessage: &waE2E.InteractiveMessage_CarouselMessage_{
			CarouselMessage: &waE2E.InteractiveMessage_CarouselMessage{
				Cards:          cards,
				MessageVersion: &messageVersion,
			},
		},
	}

	// Add body if provided (main message above carousel)
	if data.Body != "" {
		interactiveMsg.Body = &waE2E.InteractiveMessage_Body{
			Text: proto.String(data.Body),
		}
	}

	// Add footer if provided (text below carousel)
	if data.Footer != "" {
		interactiveMsg.Footer = &waE2E.InteractiveMessage_Footer{
			Text: proto.String(data.Footer),
		}
	}

	// ContextInfo is REQUIRED for iOS compatibility
	// Even if empty, iOS requires this field to display carousel
	contextInfo := &waE2E.ContextInfo{}

	// Add quoted message if exists
	if data.Quoted.MessageID != "" {
		contextInfo.StanzaID = proto.String(data.Quoted.MessageID)
		if data.Quoted.Participant != "" {
			participantJID, ok := utils.ParseJID(data.Quoted.Participant)
			if ok {
				contextInfo.Participant = proto.String(participantJID.String())
			}
		}
	}

	// Always set ContextInfo (required for iOS)
	interactiveMsg.ContextInfo = contextInfo

	// Build final message with MessageContextInfo for proper notification delivery
	msg := &waE2E.Message{
		InteractiveMessage: interactiveMsg,
		MessageContextInfo: &waE2E.MessageContextInfo{
			DeviceListMetadata: &waE2E.DeviceListMetadata{},
		},
	}

	return msg, nil
}
