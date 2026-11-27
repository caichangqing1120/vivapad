package texttools

import (
	"strings"
	"testing"
	"time"
)

func TestJSONPreservesLexemesAndSortsRecursively(t *testing.T) {
	src := ` { "z": [ {"z": 900719925474099312345, "a": -0.100e-999} ], "\u0061": "<>&\/\u0041", "a": true } `
	compact, err := MinifyJSON(src)
	if want := `{"z":[{"z":900719925474099312345,"a":-0.100e-999}],"\u0061":"<>&\/\u0041","a":true}`; err != nil || compact != want {
		t.Fatalf("minify = %q, %v", compact, err)
	}
	sorted, err := SortJSONKeys(src, "  ")
	want := "{\n  \"\\u0061\": \"<>&\\/\\u0041\",\n  \"a\": true,\n  \"z\": [\n    {\n      \"a\": -0.100e-999,\n      \"z\": 900719925474099312345\n    }\n  ]\n}"
	if err != nil || sorted != want {
		t.Fatalf("sort = %q, %v", sorted, err)
	}
	formatted, err := FormatJSON(`{"b":{}}`, "    ")
	if err != nil || formatted != "{\n    \"b\": {}\n}" {
		t.Fatalf("format = %q, %v", formatted, err)
	}
}

func TestJSONErrorCoordinates(t *testing.T) {
	for _, fixture := range []struct{ src, position string }{
		{"{\r\n  \"\u4e2d\": ]\r\n}", "line 2, column 8"},
		{"[\n", "line 2, column 1"},
		{"[] {}", "line 1, column 4"},
		{"{\r  \"a\": ]}", "line 2, column 8"},
	} {
		for _, run := range []func(string) (string, error){MinifyJSON, func(src string) (string, error) { return FormatJSON(src, "  ") }, func(src string) (string, error) { return SortJSONKeys(src, "  ") }} {
			if result, err := run(fixture.src); result != "" || err == nil || !strings.Contains(err.Error(), fixture.position) {
				t.Errorf("%q = %q, %v; want %s", fixture.src, result, err, fixture.position)
			}
		}
	}
}

func TestXMLPreservesMixedContentCDATAAndSpace(t *testing.T) {
	src := `<?xml version="1.0"?><r xmlns:p="urn:x"><p:item n='&amp;'/><m>Hello <b>world</b>!</m><s xml:space="preserve"> <a/><b xml:space="default"><a/><a/></b> </s><d><![CDATA[<a>]]></d><!--note--></r>`
	want := "<?xml version=\"1.0\"?>\n<r xmlns:p=\"urn:x\">\n  <p:item n='&amp;'/>\n  <m>Hello <b>world</b>!</m>\n  <s xml:space=\"preserve\"> <a/><b xml:space=\"default\"><a/><a/></b> </s>\n  <d><![CDATA[<a>]]></d>\n  <!--note-->\n</r>"
	got, err := FormatXML(src, "  ")
	if err != nil || got != want {
		t.Fatalf("XML = %q, %v", got, err)
	}
	if got, err := FormatXML(got, "  "); err != nil || got != want {
		t.Fatalf("reformat = %q, %v", got, err)
	}
}

func TestXMLRejectsInvalidDocuments(t *testing.T) {
	for _, src := range []string{"", "<r>", "<r></bad>", "<r/><r/>", "x<r/>", "<r a='1' a='2'/>", "<r a='1'b='2'/>", "<p:r/>", "<r>&unknown;</r>", "<r><?xml version='1.0'?></r>", "<?xml versionfoo='1.0'?><r/>", "<!DOCTYPE other><r/>"} {
		if result, err := FormatXML(src, "  "); result != "" || err == nil || !strings.Contains(err.Error(), "line ") {
			t.Errorf("invalid XML %q = %q, %v", src, result, err)
		}
	}
}

func TestXMLKeepsDeclaredEncodingOnDecodedText(t *testing.T) {
	src := `<?xml version="1.0" encoding="GB18030"?><r><name>\u4e2d</name></r>`
	src = strings.ReplaceAll(src, `\u4e2d`, "\u4e2d")
	want := "<?xml version=\"1.0\" encoding=\"GB18030\"?>\n<r>\n  <name>\u4e2d</name>\n</r>"
	if got, err := FormatXML(src, "  "); err != nil || got != want {
		t.Fatalf("XML = %q, %v; want %q", got, err, want)
	}
}

func TestConversions(t *testing.T) {
	for _, encoded := range []string{"+/8=", "+/8", "-_8=", "-_8"} {
		if got, err := Base64Decode(encoded); err != nil || got != "\xfb\xff" {
			t.Errorf("Base64 %s = %q, %v", encoded, got, err)
		}
	}
	if _, err := Base64Decode("!"); err == nil || !strings.Contains(err.Error(), "URL-safe") {
		t.Fatalf("missing Base64 hint: %v", err)
	}
	const src = "a +/%&<\u4e2d>\n"
	if got, err := Base64Decode(Base64Encode(src)); err != nil || got != src {
		t.Fatalf("Base64 roundtrip = %q, %v", got, err)
	}
	if got, err := URLDecode(URLEncode(src)); err != nil || got != src {
		t.Fatalf("URL roundtrip = %q, %v", got, err)
	}
	if _, err := URLDecode("%Q1"); err == nil {
		t.Fatal("invalid URL escape accepted")
	}
	if got := HTMLUnescape(HTMLEscape(src)); got != src {
		t.Fatalf("HTML roundtrip = %q", got)
	}
}

func TestTimestampConversion(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	for _, fixture := range []struct{ src, want string }{
		{"1700000000", "2023-11-15 06:13:20"}, {"1700000000123", "2023-11-15 06:13:20"},
		{"-1700000000000", "1916-02-18 09:46:40"},
		{"2023-11-15 06:13:20.123", "1700000000\n1700000000123"},
		{"2023-11-14T22:13:20Z", "1700000000\n1700000000000"},
	} {
		if got, err := ConvertTimestamp(fixture.src, loc); err != nil || got != fixture.want {
			t.Errorf("%q = %q, %v; want %q", fixture.src, got, err, fixture.want)
		}
	}
	for _, src := range []string{"123", "2023-02-29 00:00:00", "other"} {
		if _, err := ConvertTimestamp(src, loc); err == nil {
			t.Errorf("invalid timestamp accepted: %q", src)
		}
	}
}
