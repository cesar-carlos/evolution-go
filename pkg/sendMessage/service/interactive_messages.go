package send_service

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"github.com/gabriel-vasile/mimetype"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ValidateList checks the selection contract before any external operation.
// Explicit IDs remain unchanged; generated IDs are unique across sections.
func ValidateList(data *ListStruct) error {
	if data == nil || len(data.Sections) == 0 {
		return errors.New("at least one section is required")
	}
	ids := map[string]bool{}
	for _, section := range data.Sections {
		if len(section.Rows) == 0 {
			return errors.New("each section must contain at least one row")
		}
		for _, row := range section.Rows {
			if row.RowId != "" {
				if ids[row.RowId] {
					return errors.New("duplicate rowId")
				}
				ids[row.RowId] = true
			}
		}
	}
	return nil
}

func buildListMessage(data *ListStruct) (*waE2E.Message, error) {
	if err := ValidateList(data); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, section := range data.Sections {
		for _, row := range section.Rows {
			if row.RowId != "" {
				ids[row.RowId] = true
			}
		}
	}
	sections := make([]*waE2E.ListMessage_Section, 0, len(data.Sections))
	for i, section := range data.Sections {
		rows := make([]*waE2E.ListMessage_Row, 0, len(section.Rows))
		for j, row := range section.Rows {
			id := row.RowId
			if id == "" {
				base := fmt.Sprintf("row_%d_%d", i, j)
				id = base
				for n := 1; ids[id]; n++ {
					id = fmt.Sprintf("%s_%d", base, n)
				}
				ids[id] = true
			}
			rows = append(rows, &waE2E.ListMessage_Row{Title: proto.String(row.Title), Description: proto.String(row.Description), RowID: proto.String(id)})
		}
		sections = append(sections, &waE2E.ListMessage_Section{Title: proto.String(section.Title), Rows: rows})
	}
	text := data.ButtonText
	if text == "" {
		text = "Ver Menu"
	}
	return &waE2E.Message{ListMessage: &waE2E.ListMessage{Title: proto.String(data.Title), Description: proto.String(data.Description), FooterText: proto.String(data.FooterText), ButtonText: proto.String(text), Sections: sections, ListType: waE2E.ListMessage_SINGLE_SELECT.Enum(), ContextInfo: &waE2E.ContextInfo{}}}, nil
}

func buildButtonMessage(data *ButtonStruct) (*waE2E.Message, error) {
	if data == nil || len(data.Buttons) == 0 {
		return nil, errors.New("at least one button is required")
	}
	if err := ValidateButton(data); err != nil {
		return nil, err
	}
	hasPix := false
	for _, button := range data.Buttons {
		if button.Type == "pix" {
			hasPix = true
		}
	}
	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{}

	for _, v := range data.Buttons {
		var paramsJSON *string
		var name *string

		switch v.Type {
		case "reply":
			name = proto.String("quick_reply")
			jsonBytes, _ := json.Marshal(map[string]string{"display_text": v.DisplayText, "id": v.Id})
			paramsJSON = proto.String(string(jsonBytes))
		case "copy":
			name = proto.String("cta_copy")
			copyCode := v.CopyCode
			if copyCode == "" {
				copyCode = v.Id
			}
			copyId := v.Id
			if copyId == "" {
				copyId = "copy_" + strconv.FormatInt(time.Now().UnixNano(), 10)
			}
			jsonBytes, _ := json.Marshal(map[string]string{"display_text": v.DisplayText, "id": copyId, "copy_code": copyCode})
			paramsJSON = proto.String(string(jsonBytes))
		case "url":
			name = proto.String("cta_url")
			jsonBytes, _ := json.Marshal(map[string]string{"display_text": v.DisplayText, "url": v.URL, "merchant_url": v.URL})
			paramsJSON = proto.String(string(jsonBytes))
		case "call":
			name = proto.String("cta_call")
			jsonBytes, _ := json.Marshal(map[string]string{"display_text": v.DisplayText, "phone_number": v.PhoneNumber})
			paramsJSON = proto.String(string(jsonBytes))
		case "pix":
			randomId := utils.GenerateRandomString(11)
			name = proto.String("payment_info")
			paymentPayload := map[string]interface{}{
				"currency":     v.Currency,
				"total_amount": map[string]interface{}{"value": 0, "offset": 100},
				"reference_id": randomId,
				"type":         "physical-goods",
				"order": map[string]interface{}{
					"status":     "pending",
					"subtotal":   map[string]interface{}{"value": 0, "offset": 100},
					"order_type": "ORDER",
					"items": []map[string]interface{}{
						{
							"name":        "",
							"amount":      map[string]interface{}{"value": 0, "offset": 100},
							"quantity":    0,
							"sale_amount": map[string]interface{}{"value": 0, "offset": 100},
						},
					},
				},
				"payment_settings": []map[string]interface{}{
					{
						"type": "pix_static_code",
						"pix_static_code": map[string]string{
							"merchant_name": v.Name,
							"key":           v.Key,
							"key_type":      mapKeyType(v.KeyType),
						},
					},
				},
				"share_payment_status": false,
			}
			jsonBytes, _ := json.Marshal(paymentPayload)
			paramsJSON = proto.String(string(jsonBytes))
		default:
			return nil, errors.New("unsupported button type")
		}

		buttons = append(buttons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			Name:             name,
			ButtonParamsJSON: paramsJSON,
		})
	}

	secret := make([]byte, 32)
	if _, err := crypto_rand.Read(secret); err != nil {
		return nil, fmt.Errorf("message secret: %w", err)
	}
	body := data.Description
	messageParams := "{}"
	if hasPix {
		body = data.Title
		messageParams = `{"native_flow_name":"order_details","version":1}`
	}
	return &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
		Header: &waE2E.InteractiveMessage_Header{Title: proto.String(data.Title), HasMediaAttachment: proto.Bool(false)},
		Body:   &waE2E.InteractiveMessage_Body{Text: proto.String(body)}, Footer: &waE2E.InteractiveMessage_Footer{Text: proto.String(data.Footer)}, ContextInfo: &waE2E.ContextInfo{},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{Buttons: buttons, MessageParamsJSON: proto.String(messageParams), MessageVersion: proto.Int32(1)}},
	}, MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: secret}}, nil
}

// prepareInteractiveHeader bounds media fetching/upload and closes every body.
func prepareInteractiveHeader(ctx context.Context, client *whatsmeow.Client, imageURL, videoURL string) (*waE2E.InteractiveMessage_Header, error) {
	url := imageURL
	kind := whatsmeow.MediaImage
	if url == "" {
		url = videoURL
		kind = whatsmeow.MediaVideo
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.New("header media unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("header media returned HTTP %d", resp.StatusCode)
	}
	const maxBytes = 32 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBytes {
		return nil, errors.New("header media exceeds 32 MiB")
	}
	uploaded, err := client.Upload(ctx, raw, kind)
	if err != nil {
		return nil, err
	}
	header := &waE2E.InteractiveMessage_Header{HasMediaAttachment: proto.Bool(true)}
	if kind == whatsmeow.MediaImage {
		width, height := imageDimensions(raw)
		header.Media = &waE2E.InteractiveMessage_Header_ImageMessage{ImageMessage: &waE2E.ImageMessage{URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath), MediaKey: uploaded.MediaKey, Mimetype: proto.String(mimetype.Detect(raw).String()), FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256, FileLength: proto.Uint64(uint64(len(raw))), JPEGThumbnail: makeJPEGThumbnail(raw, 72), Width: width, Height: height}}
	} else {
		header.Media = &waE2E.InteractiveMessage_Header_VideoMessage{VideoMessage: &waE2E.VideoMessage{URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath), MediaKey: uploaded.MediaKey, Mimetype: proto.String("video/mp4"), FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256, FileLength: proto.Uint64(uint64(len(raw)))}}
	}
	return header, nil
}

// ValidateButton checks supported types and combinations before connecting.
func ValidateButton(data *ButtonStruct) error {
	if data == nil || len(data.Buttons) == 0 {
		return errors.New("at least one button is required")
	}
	hasReply := false
	hasPix := false
	hasOtherTypes := false
	replyCount := 0

	for _, v := range data.Buttons {
		switch v.Type {
		case "reply":
			hasReply = true
			replyCount++
		case "pix":
			hasPix = true
		case "url", "copy", "call":
			hasOtherTypes = true
		default:
			return errors.New("unsupported button type")
		}
	}

	if hasReply {
		if replyCount > 3 {
			return errors.New("máximo de 3 botões do tipo 'reply' permitidos")
		}
		if hasOtherTypes {
			return errors.New("botões do tipo 'reply' não podem ser misturados com outros tipos")
		}
	}

	if hasPix {
		if len(data.Buttons) > 1 {
			return errors.New("botão do tipo 'pix' não pode ser combinado com outros botões")
		}
	}

	return nil
}
