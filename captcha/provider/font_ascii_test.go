package provider

import (
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestDefaultTextFontSupportsCaptchaASCII(t *testing.T) {
	font, err := sfnt.Parse(defaultTextFont.TTF)
	if err != nil {
		t.Fatalf("parse default text font: %v", err)
	}
	var buffer sfnt.Buffer
	for _, character := range " 0123456789+-*=?ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		index, err := font.GlyphIndex(&buffer, character)
		if err != nil {
			t.Fatalf("lookup glyph %q: %v", character, err)
		}
		if index == 0 {
			t.Errorf("default text font is missing glyph %q", character)
		}
	}
}
