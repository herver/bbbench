package templating

import "testing"

func TestAllEmbeddedTemplatesParse(t *testing.T) {
	_, err := DefaultEngine()
	if err != nil {
		t.Fatalf("embedded templates failed to parse: %v", err)
	}
}
