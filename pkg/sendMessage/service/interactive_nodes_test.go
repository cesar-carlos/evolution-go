package send_service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestInteractiveSendNodesSelectionContract(t *testing.T) {
	list, err := buildListMessage(&ListStruct{Sections: []Section{{Rows: []Row{{Title: "Option", RowId: "selection-1"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	input := carouselInput()
	input.Cards[0].Buttons = []CarouselButtonStruct{{Type: "REPLY", DisplayText: "Confirm", Id: "card-1"}}
	carousel, err := buildCarouselMessage(context.Background(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		msg   *waE2E.Message
		tag   string
		attrs waBinary.Attrs
	}{
		{"list", list, "list", waBinary.Attrs{"type": "product_list", "v": "2"}},
		{"carousel", carousel, "interactive", waBinary.Attrs{"type": "native_flow", "v": "1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := proto.Clone(tt.msg)
			nodes := interactiveSendNodes(tt.msg, nil)
			if nodes == nil || len(*nodes) != 1 || (*nodes)[0].Tag != "biz" {
				t.Fatalf("missing business routing metadata: %v", nodes)
			}
			// Exercise actual binary encoding, not only the in-memory helper.
			encoded, err := waBinary.Marshal((*nodes)[0])
			if err != nil {
				t.Fatal(err)
			}
			unpacked, err := waBinary.Unpack(encoded)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := waBinary.Unmarshal(unpacked)
			if err != nil {
				t.Fatal(err)
			}
			children := decoded.GetChildren()
			if len(children) != 2 || children[0].Tag != tt.tag || !reflect.DeepEqual(children[0].Attrs, tt.attrs) {
				t.Fatalf("incorrect wire metadata: %#v", children)
			}
			if tt.name == "carousel" {
				flow := children[0].GetChildren()
				if len(flow) != 1 || flow[0].Tag != "native_flow" || flow[0].Attrs["name"] != "mixed" || flow[0].Attrs["v"] != "9" {
					t.Fatalf("missing carousel native flow: %#v", flow)
				}
			}
			if children[1].Tag != "quality_control" || children[1].Attrs["source_type"] != "third_party" {
				t.Fatalf("missing third-party metadata: %#v", children[1])
			}
			if !proto.Equal(before, tt.msg) {
				t.Fatal("encrypted payload/context mutated")
			}
		})
	}
	if list.ListMessage.GetListType() != waE2E.ListMessage_SINGLE_SELECT || list.ListMessage.Sections[0].Rows[0].GetRowID() != "selection-1" {
		t.Fatal("list selection contract changed")
	}
	var params map[string]string
	if err := json.Unmarshal([]byte(carousel.GetInteractiveMessage().GetCarouselMessage().Cards[0].GetNativeFlowMessage().Buttons[0].GetButtonParamsJSON()), &params); err != nil || params["id"] != "card-1" {
		t.Fatalf("carousel reply ID changed: %v, %v", params, err)
	}
}

func TestInteractiveSendNodesPreservesOverridesAndOtherMessages(t *testing.T) {
	list := &waE2E.Message{ListMessage: &waE2E.ListMessage{ListType: waE2E.ListMessage_SINGLE_SELECT.Enum()}}
	for _, tag := range []string{"biz", "hsm"} {
		extra := []waBinary.Node{{Tag: tag, Attrs: waBinary.Attrs{"custom": "yes"}}}
		if got := interactiveSendNodes(list, &extra); got != &extra {
			t.Fatal("explicit override replaced")
		}
	}
	extra := make([]waBinary.Node, 1, 2)
	extra[0] = waBinary.Node{Tag: "meta", Attrs: waBinary.Attrs{"custom": "yes"}}
	nodes := interactiveSendNodes(list, &extra)
	if len(*nodes) != 2 || !reflect.DeepEqual((*nodes)[0], extra[0]) || extra[:2][1].Tag != "" {
		t.Fatal("caller metadata lost or backing array mutated")
	}
	for _, msg := range []*waE2E.Message{
		nil, {}, {Conversation: proto.String("text")},
		{ListMessage: &waE2E.ListMessage{ListType: waE2E.ListMessage_PRODUCT_LIST.Enum()}},
		{InteractiveMessage: &waE2E.InteractiveMessage{InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{}}}},
		{ListResponseMessage: &waE2E.ListResponseMessage{}},
		{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{}},
		{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}},
		{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{}},
	} {
		if interactiveSendNodes(msg, nil) != nil || interactiveSendNodes(msg, &extra) != &extra {
			t.Fatalf("unrelated message changed: %v", msg)
		}
	}
}
