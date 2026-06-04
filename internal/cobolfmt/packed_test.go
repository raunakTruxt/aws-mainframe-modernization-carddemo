package cobolfmt_test

import (
	"bytes"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/cobolfmt"
	"github.com/shopspring/decimal"
)

func TestPackedSize(t *testing.T) {
	tests := []struct {
		digits int
		want   int
	}{
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{5, 3},
		{6, 4},
		{7, 4},
		{9, 5},
		{11, 6},
	}
	for _, tc := range tests {
		got := cobolfmt.PackedSize(tc.digits)
		if got != tc.want {
			t.Errorf("PackedSize(%d) = %d, want %d", tc.digits, got, tc.want)
		}
	}
}

func TestDecodePacked(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		scale   int
		want    string
		wantErr bool
	}{
		{
			name:  "positive 123 (3 digits, 2 bytes, sign C)",
			input: []byte{0x01, 0x2C}, // digits 0,1,2 sign=C(+) -> 12
			scale: 0,
			want:  "12",
		},
		{
			name:  "unsigned 123 (3 digits, 2 bytes, sign F)",
			input: []byte{0x01, 0x2F}, // digits 0,1,2 sign=F(unsigned) -> 12
			scale: 0,
			want:  "12",
		},
		{
			name:  "negative 456 (sign D)",
			input: []byte{0x04, 0x5D}, // digits 0,4,5 sign=D(-) -> -45
			scale: 0,
			want:  "-45",
		},
		{
			name:  "zero positive (sign C)",
			input: []byte{0x0C},
			scale: 0,
			want:  "0",
		},
		{
			// negative-zero: 0xD sign over zero digits decodes to 0 (sign info lost).
			// Re-encoding as signed will produce 0xC, not 0xD — known shopspring limitation.
			name:  "negative-zero (sign D, all-zero digits) decodes to 0",
			input: []byte{0x00, 0x0D},
			scale: 0,
			want:  "0",
		},
		{
			name:  "positive with scale 2",
			input: []byte{0x01, 0x23, 0x4C}, // digits 0,1,2,3,4 sign C -> 12.34
			scale: 2,
			want:  "12.34",
		},
		{
			name:  "negative with scale 2",
			input: []byte{0x00, 0x05, 0x0D}, // digits 0,0,0,5,0 sign D -> -0.50
			scale: 2,
			want:  "-0.50",
		},
		{
			name:    "invalid sign nibble",
			input:   []byte{0x01, 0x2B},
			scale:   0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cobolfmt.DecodePacked(tc.input, tc.scale)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want, _ := decimal.NewFromString(tc.want)
			if !got.Equal(want) {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}

func TestEncodePackedUnsigned(t *testing.T) {
	// unsigned=true must produce 0xF sign nibble regardless of value sign.
	tests := []struct {
		name     string
		value    string
		digits   int
		scale    int
		wantHex  []byte
	}{
		{
			name:    "unsigned 123 (3 digits) -> 12 3F",
			value:   "123",
			digits:  3,
			scale:   0,
			wantHex: []byte{0x12, 0x3F},
		},
		{
			name:    "unsigned 0 (3 digits) -> 00 0F",
			value:   "0",
			digits:  3,
			scale:   0,
			wantHex: []byte{0x00, 0x0F},
		},
		{
			name:    "unsigned 300 (3 digits) -> 30 0F",
			value:   "300",
			digits:  3,
			scale:   0,
			wantHex: []byte{0x30, 0x0F},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := decimal.NewFromString(tc.value)
			encoded, err := cobolfmt.EncodePacked(d, tc.digits, tc.scale, true)
			if err != nil {
				t.Fatalf("EncodePacked: %v", err)
			}
			if !bytes.Equal(encoded, tc.wantHex) {
				t.Errorf("got %X, want %X", encoded, tc.wantHex)
			}
		})
	}
}

func TestEncodePackedUnsignedRoundTrip(t *testing.T) {
	// Unsigned round-trip: encode with 0xF, decode (accepts 0xF as positive).
	values := []string{"0", "1", "123", "300", "776", "999"}
	for _, v := range values {
		t.Run(v, func(t *testing.T) {
			d, _ := decimal.NewFromString(v)
			encoded, err := cobolfmt.EncodePacked(d, 3, 0, true)
			if err != nil {
				t.Fatalf("EncodePacked: %v", err)
			}
			// Sign nibble must be 0xF.
			if encoded[len(encoded)-1]&0x0F != 0xF {
				t.Errorf("sign nibble: got 0x%X, want 0xF", encoded[len(encoded)-1]&0x0F)
			}
			decoded, err := cobolfmt.DecodePacked(encoded, 0)
			if err != nil {
				t.Fatalf("DecodePacked: %v", err)
			}
			if !decoded.Equal(d) {
				t.Errorf("round-trip: got %s, want %s", decoded, d)
			}
		})
	}
}

func TestEncodePackedRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		digits   int
		scale    int
		unsigned bool
	}{
		{"positive 0 signed", "0", 3, 0, false},
		{"positive 123 signed", "123", 3, 0, false},
		{"negative 45 signed", "-45", 3, 0, false},
		{"positive 12.34 signed", "12.34", 5, 2, false},
		{"negative 0.50 signed", "-0.50", 3, 2, false},
		{"odd digits 9 signed", "9", 1, 0, false},
		{"even digits 99 signed", "99", 2, 0, false},
		{"large 9999999.99 signed", "9999999.99", 9, 2, false},
		{"unsigned 123", "123", 3, 0, true},
		{"unsigned 0", "0", 3, 0, true},
		{"unsigned 999", "999", 3, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := decimal.NewFromString(tc.value)
			if err != nil {
				t.Fatalf("invalid decimal: %v", err)
			}

			encoded, err := cobolfmt.EncodePacked(d, tc.digits, tc.scale, tc.unsigned)
			if err != nil {
				t.Fatalf("EncodePacked: %v", err)
			}
			wantSize := cobolfmt.PackedSize(tc.digits)
			if len(encoded) != wantSize {
				t.Fatalf("encoded length: got %d, want %d", len(encoded), wantSize)
			}

			decoded, err := cobolfmt.DecodePacked(encoded, tc.scale)
			if err != nil {
				t.Fatalf("DecodePacked: %v", err)
			}
			if !decoded.Equal(d) {
				t.Errorf("round-trip: got %s, want %s", decoded, d)
			}
		})
	}
}

func TestEncodePackedSignNibble(t *testing.T) {
	// Verify exact sign nibble values for signed vs unsigned encoding.
	d := decimal.NewFromInt(5)
	neg := decimal.NewFromInt(-5)

	tests := []struct {
		name      string
		d         decimal.Decimal
		unsigned  bool
		wantNibble byte
	}{
		{"positive signed -> 0xC", d, false, 0xC},
		{"negative signed -> 0xD", neg, false, 0xD},
		{"positive unsigned -> 0xF", d, true, 0xF},
		{"zero signed -> 0xC", decimal.Zero, false, 0xC},
		{"zero unsigned -> 0xF", decimal.Zero, true, 0xF},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := cobolfmt.EncodePacked(tc.d, 1, 0, tc.unsigned)
			if err != nil {
				t.Fatalf("EncodePacked: %v", err)
			}
			got := encoded[len(encoded)-1] & 0x0F
			if got != tc.wantNibble {
				t.Errorf("sign nibble: got 0x%X, want 0x%X", got, tc.wantNibble)
			}
		})
	}
}
