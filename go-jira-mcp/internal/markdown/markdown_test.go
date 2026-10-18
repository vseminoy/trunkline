package markdown

import "testing"

func TestParsePrefix(t *testing.T) {
	p, err := ParsePrefix("C2-fix-export.md")
	if err != nil {
		t.Fatal(err)
	}
	if p.Raw != "C2" || p.Letter != "C" || p.Num != 2 {
		t.Fatalf("got %+v", p)
	}
	if !p.Less(Prefix{Letter: "C", Num: 10, Raw: "C10"}) {
		t.Fatal("C2 should be less than C10")
	}
}

func TestPrepareDescription(t *testing.T) {
	body := "see ![err](screens/err.png)\n"
	prep, err := PrepareDescription(body)
	if err != nil {
		t.Fatal(err)
	}
	if prep.Text != "see !err.png!\n" {
		t.Fatalf("text: %q", prep.Text)
	}
	if len(prep.Images) != 1 || prep.Images[0] != "screens/err.png" {
		t.Fatalf("images: %v", prep.Images)
	}
}

func TestSplitFrontMatter(t *testing.T) {
	raw := []byte("---\nproject: BR\nsummary: Test\nready: true\ntype: Task\n---\n## Проблема\nx\n")
	fm, body, err := splitFrontMatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRequired(fm); err != nil {
		t.Fatal(err)
	}
	if body != "## Проблема\nx\n" {
		t.Fatalf("body: %q", body)
	}
}
