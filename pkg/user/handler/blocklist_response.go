package user_handler

// BlocklistResponse describes the blocklist envelope since the 0.7.3 base.
// Items replaces JIDs and carries both phone-number and LID identities.
type BlocklistResponse struct {
	Message string         `json:"message"`
	Data    *BlocklistData `json:"data"`
}

// BlocklistData documents the dependency's exported JSON names explicitly.
// Its JIDs marshal as strings, even though the library uses a struct internally.
type BlocklistData struct {
	AddressingMode string          `json:"AddressingMode"`
	DHash          string          `json:"DHash"`
	Items          []BlocklistItem `json:"Items"`
}

// BlocklistItem identifies one entry using the phone number and LID from WhatsApp.
type BlocklistItem struct {
	LID    string `json:"LID"`
	PN     string `json:"PN"`
	Active bool   `json:"Active"`
}
