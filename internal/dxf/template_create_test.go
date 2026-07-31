package dxf

import "testing"

func TestNormalizeLayerByEntity(t *testing.T) {
	tests := []struct {
		layer    string
		entity   string
		expected string
	}{
		// ATTRIB: N_COM_HIDDEN → 0
		{"1_COM_HIDDEN", "ATTRIB", "0"},
		{"2_COM_HIDDEN", "ATTRIB", "0"},
		{"3_COM_HIDDEN", "ATTRIB", "0"},
		{"4_COM_HIDDEN", "ATTRIB", "0"},
		// ATTRIB: N_COM_COM_HIDDEN stays as-is
		{"1_COM_COM_HIDDEN", "ATTRIB", "1_COM_COM_HIDDEN"},
		// ATTRIB: N_COM_COM_EVAL_FALSE → N_COM_COM_HIDDEN
		{"1_COM_COM_EVAL_FALSE", "ATTRIB", "1_COM_COM_HIDDEN"},
		// ATTRIB: non-COM layers unchanged
		{"0", "ATTRIB", "0"},
		{"1", "ATTRIB", "1"},

		// SEQEND: N_COM_EVAL_FALSE → N_COM_HIDDEN
		{"1_COM_EVAL_FALSE", "SEQEND", "1_COM_HIDDEN"},
		{"2_COM_EVAL_FALSE", "SEQEND", "2_COM_HIDDEN"},
		{"3_COM_EVAL_FALSE", "SEQEND", "3_COM_HIDDEN"},
		// SEQEND: N_COM_HIDDEN stays as-is
		{"1_COM_HIDDEN", "SEQEND", "1_COM_HIDDEN"},
		// SEQEND: N_COM_COM_EVAL_FALSE → N_COM_COM_HIDDEN
		{"1_COM_COM_EVAL_FALSE", "SEQEND", "1_COM_COM_HIDDEN"},
		// SEQEND: non-COM layers unchanged
		{"1", "SEQEND", "1"},
		{"0", "SEQEND", "0"},

		// INSERT: no normalization (leave as-is)
		{"1_COM_HIDDEN", "INSERT", "1_COM_HIDDEN"},
		// INSERT: N_COM_COM_HIDDEN stays
		{"1_COM_COM_HIDDEN", "INSERT", "1_COM_COM_HIDDEN"},

		// Unknown entity: no change
		{"1_COM_HIDDEN", "LINE", "1_COM_HIDDEN"},
		{"0", "", "0"},
	}

	for _, tt := range tests {
		got := normalizeLayerByEntity(tt.layer, tt.entity)
		if got != tt.expected {
			t.Errorf("normalizeLayerByEntity(%q, %q) = %q, want %q",
				tt.layer, tt.entity, got, tt.expected)
		}
	}
}

func TestExtractModuleID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"C:\\Users\\Public\\output\\AMR02p3\\602_p5000_p0006.dxf", "602.5000.0006"},
		{"C:\\Users\\Public\\output\\BO002p1\\745_p0010_p1100.dxf", "745.0010.1100"},
		{"C:\\Users\\Public\\output\\AU_FCUVLVCNTRLp5\\AU_c631_p1811_p0061.dxf", "631.1811.0061"},
		{"C:\\Users\\Public\\output\\MF001p2n1\\635_p1330.dxf", "635.1330"},
	}

	for _, tt := range tests {
		got := extractModuleID(tt.input)
		if got != tt.expected {
			t.Errorf("extractModuleID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}