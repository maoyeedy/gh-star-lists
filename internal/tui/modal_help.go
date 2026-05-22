package tui

func newHelpModal() *modal {
	return &modal{
		kind:  modalHelp,
		title: "Key Bindings",
	}
}
