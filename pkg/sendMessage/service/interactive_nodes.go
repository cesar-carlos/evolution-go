package send_service

import (
	"encoding/hex"
	"strconv"
	"time"

	"go.mau.fi/util/random"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

// interactiveSendNodes supplies transport metadata missing from the pinned
// Whatsmeow fork. Its AdditionalNodes API gives explicit biz/hsm nodes precedence
// over automatic metadata, without changing the encrypted selection payload.
// Keep this scoped to the direct prompts produced by our list/carousel builders.
func interactiveSendNodes(msg *waE2E.Message, additional *[]waBinary.Node) *[]waBinary.Node {
	if msg == nil || msg.ButtonsResponseMessage != nil || msg.ListResponseMessage != nil || msg.InteractiveResponseMessage != nil || msg.TemplateButtonReplyMessage != nil {
		return additional
	}
	var prompt waBinary.Node
	switch {
	case msg.GetListMessage().GetListType() == waE2E.ListMessage_SINGLE_SELECT:
		// The wire routing type differs from the protobuf selection type:
		// evolution-foundation/evolution-go#204 and tulir/whatsmeow#1146.
		prompt = waBinary.Node{Tag: "list", Attrs: waBinary.Attrs{"type": "product_list", "v": "2"}}
	case msg.GetInteractiveMessage().GetCarouselMessage() != nil:
		prompt = waBinary.Node{Tag: "interactive", Attrs: waBinary.Attrs{"type": "native_flow", "v": "1"}, Content: []waBinary.Node{
			{Tag: "native_flow", Attrs: waBinary.Attrs{"name": "mixed", "v": "9"}},
		}}
	default:
		return additional
	}
	var nodes []waBinary.Node
	if additional != nil {
		for _, node := range *additional {
			if node.Tag == "biz" || node.Tag == "hsm" {
				return additional
			}
		}
		// Do not append into caller-owned storage.
		nodes = append(nodes, (*additional)...)
	}
	nodes = append(nodes, waBinary.Node{
		Tag:   "biz",
		Attrs: waBinary.Attrs{"actual_actors": "2", "host_storage": "2", "privacy_mode_ts": strconv.FormatInt(time.Now().Unix(), 10)},
		Content: []waBinary.Node{
			prompt,
			{Tag: "quality_control", Attrs: waBinary.Attrs{"decision_id": hex.EncodeToString(random.Bytes(20)), "source_type": "third_party"}, Content: []waBinary.Node{
				{Tag: "decision_source", Attrs: waBinary.Attrs{"value": "df"}},
			}},
		},
	})
	return &nodes
}
