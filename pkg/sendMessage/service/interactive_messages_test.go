package send_service

import (
	"encoding/json"
	"testing"
)

func TestListSelectionIDs(t *testing.T) {
	input := &ListStruct{Sections: []Section{{Rows: []Row{{Title: "a"}, {Title: "b", RowId: "row_1_0"}}}, {Rows: []Row{{Title: "c"}}}}}
	msg, err := buildListMessage(input)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, section := range msg.ListMessage.Sections {
		for _, row := range section.Rows {
			id := row.GetRowID()
			if id == "" || seen[id] {
				t.Fatalf("ambiguous selection %q", id)
			}
			seen[id] = true
		}
	}
	if msg.ListMessage.Sections[0].Rows[1].GetRowID() != "row_1_0" {
		t.Fatal("explicit ID changed")
	}
	if input.Sections[1].Rows[0].RowId != "" {
		t.Fatal("input mutated")
	}
	if msg.ListMessage.GetButtonText() != "Ver Menu" {
		t.Fatal("default label missing")
	}
	for _, bad := range []*ListStruct{nil, {}, {Sections: []Section{{}}}, {Sections: []Section{{Rows: []Row{{RowId: "same"}, {RowId: "same"}}}}}} {
		if _, err := buildListMessage(bad); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
}

func TestButtonNativeFlowAndValidation(t *testing.T) {
	for _, kind := range []string{"reply", "url", "copy", "call", "pix"} {
		t.Run(kind, func(t *testing.T) {
			msg, err := buildButtonMessage(&ButtonStruct{Title: "T", Description: "D", Footer: "F", Buttons: []Button{{Type: kind, Id: "id\"\\", DisplayText: "label\"\\", URL: "https://example.org", CopyCode: "code"}}})
			if err != nil {
				t.Fatal(err)
			}
			if msg.DocumentWithCaptionMessage != nil || msg.ButtonsMessage != nil {
				t.Fatal("legacy wrapper retained")
			}
			nf := msg.GetInteractiveMessage().GetNativeFlowMessage()
			if nf == nil || len(nf.Buttons) != 1 {
				t.Fatal("missing native flow")
			}
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(nf.Buttons[0].GetButtonParamsJSON()), &params); err != nil {
				t.Fatal(err)
			}
			if kind == "reply" && params["id"] != "id\"\\" {
				t.Fatal("reply ID changed")
			}
			if len(msg.GetMessageContextInfo().GetMessageSecret()) != 32 {
				t.Fatal("missing secret")
			}
		})
	}
	for _, buttons := range [][]Button{nil, {{Type: "unknown"}}, {{Type: "reply"}, {Type: "url"}}, {{Type: "pix"}, {Type: "call"}}, {{Type: "reply"}, {Type: "reply"}, {Type: "reply"}, {Type: "reply"}}} {
		if _, err := buildButtonMessage(&ButtonStruct{Buttons: buttons}); err == nil {
			t.Fatal("invalid buttons accepted")
		}
	}
}
