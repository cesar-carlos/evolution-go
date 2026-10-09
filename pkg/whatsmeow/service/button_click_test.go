package whatsmeow_service

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestButtonClickFormats(t *testing.T) {
	native := func(params string) *waE2E.Message {
		return &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{Name: proto.String("quick_reply"), ParamsJSON: proto.String(params)}}}}
	}
	cases := []struct {
		msg      *waE2E.Message
		id, kind string
	}{
		{&waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("b")}}, "b", "buttons_response"},
		{&waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String("r")}}}, "r", "list_response"},
		{&waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{SelectedID: proto.String("t")}}, "t", "template_button_reply"},
		{native(`{"id":"n","display_text":"N"}`), "n", "native_flow_response"},
		{native(`{"selected_row_id":"row_1_0"}`), "row_1_0", "native_flow_response"},
	}
	for _, tt := range cases {
		before := proto.Clone(tt.msg)
		result := parseButtonClick(&waE2E.Message{DocumentWithCaptionMessage: &waE2E.FutureProofMessage{Message: tt.msg}})
		if result["buttonId"] != tt.id || result["type"] != tt.kind {
			t.Fatalf("wrong click: %#v", result)
		}
		if !proto.Equal(tt.msg, before) {
			t.Fatal("event mutated")
		}
	}
	for _, msg := range []*waE2E.Message{nil, {}, {DocumentWithCaptionMessage: &waE2E.FutureProofMessage{}}} {
		if parseButtonClick(msg) != nil {
			t.Fatal("spurious click")
		}
	}
	if got := parseButtonClick(native(`invalid`)); got["buttonId"] != "" {
		t.Fatal("invalid JSON produced ID")
	}
}
