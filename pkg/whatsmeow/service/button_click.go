package whatsmeow_service

import (
	"encoding/json"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

// parseButtonClick reads a response without mutating the event shared by consumers.
func parseButtonClick(message *waE2E.Message) map[string]interface{} {
	for depth := 0; message != nil; depth++ {
		if depth >= 32 {
			return nil
		}
		switch {
		case message.DocumentWithCaptionMessage != nil:
			message = message.DocumentWithCaptionMessage.Message
		case message.EphemeralMessage != nil:
			message = message.EphemeralMessage.Message
		case message.ViewOnceMessage != nil:
			message = message.ViewOnceMessage.Message
		case message.ViewOnceMessageV2 != nil:
			message = message.ViewOnceMessageV2.Message
		case message.ViewOnceMessageV2Extension != nil:
			message = message.ViewOnceMessageV2Extension.Message
		default:
			goto parsed
		}
	}
parsed:
	if message == nil {
		return nil
	}
	var buttonClickData map[string]interface{}

	if resp := message.GetButtonsResponseMessage(); resp != nil {
		// Legacy buttons response
		buttonClickData = map[string]interface{}{
			"buttonId":   resp.GetSelectedButtonID(),
			"buttonText": resp.GetSelectedDisplayText(),
			"type":       "buttons_response",
		}
	} else if resp := message.GetInteractiveResponseMessage(); resp != nil {
		// NativeFlow interactive response (quick_reply, cta_url, cta_call, cta_copy)
		if nf := resp.GetNativeFlowResponseMessage(); nf != nil {
			buttonId := ""
			buttonText := ""
			// Parse paramsJSON to extract id and display_text
			if nf.GetParamsJSON() != "" {
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(nf.GetParamsJSON()), &params); err == nil {
					if id, ok := params["id"].(string); ok {
						buttonId = id
					}
					if buttonId == "" {
						if id, ok := params["selected_row_id"].(string); ok {
							buttonId = id
						}
					}
					if dt, ok := params["display_text"].(string); ok {
						buttonText = dt
					}
				}
			}
			buttonClickData = map[string]interface{}{
				"buttonId":   buttonId,
				"buttonText": buttonText,
				"type":       "native_flow_response",
				"name":       nf.GetName(),
				"paramsJSON": nf.GetParamsJSON(),
			}
		}
	} else if resp := message.GetTemplateButtonReplyMessage(); resp != nil {
		// Template button reply
		buttonClickData = map[string]interface{}{
			"buttonId":   resp.GetSelectedID(),
			"buttonText": resp.GetSelectedDisplayText(),
			"type":       "template_button_reply",
		}
	} else if resp := message.GetListResponseMessage(); resp != nil {
		// List response (single select)
		buttonClickData = map[string]interface{}{
			"buttonId":    resp.GetSingleSelectReply().GetSelectedRowID(),
			"buttonText":  resp.GetTitle(),
			"type":        "list_response",
			"description": resp.GetDescription(),
		}
	}
	return buttonClickData
}
