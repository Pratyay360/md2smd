package smd

import (
	"strings"
	"testing"
)

func TestMdToSmd(t *testing.T) {
	tests := []struct {
		name string
		in   string
		// wantBody is matched exactly against the document after the
		// frontmatter block; frontmatter itself is checked via wantFM.
		wantBody string
		wantFM   []string
		notFM    []string
	}{
		{
			name:     "image becomes a directive",
			in:       "![alt](https://example.com/a.png)\n",
			wantBody: "[alt]($image.url(\"https://example.com/a.png\"))\n",
			wantFM:   []string{".layout", ".date", ".title"},
		},
		{
			name:     "image title is carried as alt",
			in:       "![pic](a.png \"The Alt\")\n",
			wantBody: "[pic]($image.asset(\"a.png\").alt(\"The Alt\"))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "root-relative image uses siteAsset",
			in:       "![](/img/a.png)\n",
			wantBody: "[]($image.siteAsset(\"/img/a.png\"))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "relative image uses asset",
			in:       "![x](img/a.png)\n",
			wantBody: "[x]($image.asset(\"img/a.png\"))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "external link",
			in:       "[t](https://x.com)\n",
			wantBody: "[t]($link.url(\"https://x.com\").new(true))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "anchor link",
			in:       "[t](#a)\n",
			wantBody: "[t]($link.ref(\"a\"))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "sub link",
			in:       "[t](./s)\n",
			wantBody: "[t]($link.sub(\"s\"))\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "code blocks are left alone",
			in:       "```\n![a](b.png)\n```\n",
			wantBody: "```\n![a](b.png)\n```\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "frontmatter becomes ziggy",
			in:       "---\ntitle: Hi\n---\n\nbody\n",
			wantBody: "\nbody\n",
			wantFM:   []string{".title = \"Hi\"", ".layout", ".date"},
		},
		{
			name:     "asciinema badge becomes player embed",
			in:       "[![asciicast](https://asciinema.org/a/abc.svg)](https://asciinema.org/a/abc)\n",
			wantBody: "```=html\n<script src=\"https://asciinema.org/a/abc.js\" id=\"asciicast-abc\" async></script>\n```\n",
			wantFM:   []string{".layout", ".date"},
		},
		{
			name:     "hugo headless and weight move to custom",
			in:       "---\ntitle: T\nheadless: false\nweight: 0\n---\n\nbody\n",
			wantBody: "\nbody\n",
			wantFM:   []string{".title = \"T\"", ".custom.headless = false", ".custom.weight = 0"},
			notFM:    []string{"\n.headless =", "\n.weight ="},
		},
		{
			name:     "headless true implies draft",
			in:       "---\ntitle: T\nheadless: true\n---\n\nbody\n",
			wantBody: "\nbody\n",
			wantFM:   []string{".draft = true", ".custom.headless = true"},
			notFM:    []string{"\n.headless ="},
		},
		{
			name:     "author and categories map to zine fields",
			in:       "---\ntitle: T\nauthor: Jane\ncategories: [News]\ntags: [Go]\n---\n\nbody\n",
			wantBody: "\nbody\n",
			wantFM:   []string{".authors = [\"Jane\"]", "\"News\"", "\"Go\""},
			notFM:    []string{"\n.author =", "\n.categories =", "\n.category ="},
		},
		{
			name:     "headings starting at h3 shift to h1",
			in:       "### Hello\n\n### World\n",
			wantBody: "# Hello\n\n# World\n",
			wantFM:   []string{".title = \"Hello\""},
		},
		{
			name:     "skipped heading levels are clamped",
			in:       "# Top\n\n### Deep\n",
			wantBody: "# Top\n\n## Deep\n",
			wantFM:   []string{".title = \"Top\""},
		},
		{
			name:     "headings in code blocks are untouched",
			in:       "# Top\n\n```\n### not a heading\n```\n",
			wantBody: "# Top\n\n```\n### not a heading\n```\n",
			wantFM:   []string{".title = \"Top\""},
		},
		{
			name:     "doc opening with h3 still starts at h1",
			in:       "### First\n\n# Second\n\n### Third\n",
			wantBody: "# First\n\n# Second\n\n# Third\n",
			wantFM:   []string{".title = \"First\""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MdToSmd(tt.in)
			if err != nil {
				t.Fatalf("MdToSmd() error = %v", err)
			}
			fm, body := splitSmd(got)
			if body != tt.wantBody {
				t.Errorf("MdToSmd() body\n got = %q\nwant = %q\nfull = %q", body, tt.wantBody, got)
			}
			for _, want := range tt.wantFM {
				if !strings.Contains(fm, want) {
					t.Errorf("MdToSmd() frontmatter missing %q\n got fm = %q\nfull = %q", want, fm, got)
				}
			}
			for _, not := range tt.notFM {
				if strings.Contains(fm, not) {
					t.Errorf("MdToSmd() frontmatter must not contain %q\n got fm = %q", not, fm)
				}
			}
		})
	}
}

// splitSmd divides an .smd document into its frontmatter block and body.
func splitSmd(doc string) (fm, body string) {
	if !strings.HasPrefix(doc, "---\n") {
		return "", doc
	}
	rest := doc[len("---\n"):]
	idx := strings.Index(rest, "---\n")
	if idx < 0 {
		return "", doc
	}
	return rest[:idx], rest[idx+len("---\n"):]
}

func TestSmdToMd(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "image directive",
			in:   "[alt]($image.url(\"https://x.com/a.png\").alt(\"alt\"))\n",
			want: "![alt](https://x.com/a.png \"alt\")\n",
		},
		{
			name: "empty caption image",
			in:   "[]($image.asset(\"a.png\"))\n",
			want: "![](a.png)\n",
		},
		{
			name: "link directive",
			in:   "[t]($link.page(\"about\"))\n",
			want: "[t](/about)\n",
		},
		{
			name: "site link resolves to root",
			in:   "[t]($link.site())\n",
			want: "[t](/)\n",
		},
		{
			name: "player embed returns to badge",
			in:   "```=html\n<script src=\"https://asciinema.org/a/abc.js\" id=\"asciicast-abc\" async></script>\n```\n",
			want: "[![asciicast](https://asciinema.org/a/abc.svg)](https://asciinema.org/a/abc)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SmdToMd(tt.in)
			if err != nil {
				t.Fatalf("SmdToMd() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("SmdToMd()\n got = %q\nwant = %q", got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct{ in, smdBody, wantBack string }{
		{"![alt](https://example.com/a.png)\n", "[alt]($image.url(\"https://example.com/a.png\"))\n", ""},
		{"[t](https://x.com)\n", "[t]($link.url(\"https://x.com\").new(true))\n", ""},
		{"[t](#a)\n", "[t]($link.ref(\"a\"))\n", ""},
		{"![](/img/a.png)\n", "[]($image.siteAsset(\"/img/a.png\"))\n", ""},
		{"```\n![a](b.png)\n```\n", "```\n![a](b.png)\n```\n", ""},
		{"[![asciicast](https://asciinema.org/a/abc.svg)](https://asciinema.org/a/abc)\n", "```=html\n<script src=\"https://asciinema.org/a/abc.js\" id=\"asciicast-abc\" async></script>\n```\n", ""},
		{"![pic](a.png \"The Alt\")\n", "[pic]($image.asset(\"a.png\").alt(\"The Alt\"))\n", ""},
		{"[t](./s)\n", "[t]($link.sub(\"s\"))\n", ""},
		{"# Hi\n", "# Hi\n", ""},
		// Heading normalization is intentionally lossy: ### shifts to #.
		{"### Shifted\n", "# Shifted\n", "# Shifted\n"},
	}
	for _, tt := range tests {
		smd, err := MdToSmd(tt.in)
		if err != nil {
			t.Fatalf("MdToSmd(%q) error = %v", tt.in, err)
		}
		_, smdBody := splitSmd(smd)
		if smdBody != tt.smdBody {
			t.Errorf("MdToSmd(%q) body = %q, want %q", tt.in, smdBody, tt.smdBody)
		}
		got, err := SmdToMd(smd)
		if err != nil {
			t.Fatalf("SmdToMd(%q) error = %v", smd, err)
		}
		_, mdBody := splitSmd(got)
		// SmdToMd preserves the body; frontmatter is expected to
		// gain title/date/layout defaults, so only the body round-trips.
		wantBody := tt.wantBack
		if wantBody == "" {
			wantBody = tt.in
		}
		if strings.HasPrefix(mdBody, "\n") && !strings.HasPrefix(wantBody, "\n") {
			wantBody = "\n" + wantBody
		}
		if mdBody != wantBody {
			t.Errorf("round trip body\n  in:   %q\n  smd:  %q\n  back: %q\n  want: %q", tt.in, smd, got, wantBody)
		}
	}
}

// TestLossyRoundTrip pins down conversions that are deliberately not lossless.
// SuperMD has no representation for a bare linked image's caption distinct from
// the inner image alt, and Ziggy frontmatter re-serialisation normalises the
// blank line after the closing fence (plus title/date/layout defaults).
func TestLossyRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantBody string
		wantFM   []string
	}{
		{
			name:     "linked image caption becomes the inner alt",
			in:       "[![cap](https://x.com/i.png)](https://y.com)\n",
			wantBody: "\n[![cap](https://x.com/i.png \"cap\")](https://y.com)\n",
			wantFM:   []string{"title:", "layout:"},
		},
		{
			name:     "frontmatter gains defaults and a trailing blank line",
			in:       "---\ntitle: Hi\n---\n\n# H\n",
			wantBody: "\n\n# H\n",
			wantFM:   []string{"title: Hi", "layout:", "date:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			smd, err := MdToSmd(tt.in)
			if err != nil {
				t.Fatalf("MdToSmd() error = %v", err)
			}
			got, err := SmdToMd(smd)
			if err != nil {
				t.Fatalf("SmdToMd() error = %v", err)
			}
			fm, body := splitSmd(got)
			_ = fm
			if body != tt.wantBody {
				t.Errorf("round trip body\n  in:   %q\n  smd:  %q\n  back: %q\n  want body: %q", tt.in, smd, got, tt.wantBody)
			}
			for _, want := range tt.wantFM {
				if !strings.Contains(got, want) {
					t.Errorf("round trip missing %q\n  back = %q", want, got)
				}
			}
		})
	}
}

func TestFencedBlocksRoundTrip(t *testing.T) {
	in := "before\n```js\n[a](b)\n```\nafter\n~~~py\n![x](y)\n~~~\n"
	stripped, blocks := extractFencedBlocks(in)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if got := restoreBlocks(stripped, blocks); got != in {
		t.Errorf("restoreBlocks()\n got = %q\nwant = %q", got, in)
	}
}

func TestParseDirectiveCalls(t *testing.T) {
	name, calls := parseDirectiveCalls(`$image.asset("a.png").alt("hi")`)
	if name != "image" {
		t.Errorf("directive name = %q, want image", name)
	}
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0].function != "asset" || unquote(calls[0].args[0]) != "a.png" {
		t.Errorf("call 0 = %+v", calls[0])
	}
	if calls[1].function != "alt" || unquote(calls[1].args[0]) != "hi" {
		t.Errorf("call 1 = %+v", calls[1])
	}
}

func TestSplitScriptyArgsKeepsQuotedCommas(t *testing.T) {
	got := splitScriptyArgs(`"a,b", "c", "d,e"`)
	want := []string{`"a,b"`, `"c"`, `"d,e"`}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestZiggyFrontmatterRoundTrip(t *testing.T) {
	fm := ".title = \"My Post\",\n.date = .date(\"2024-01-15\"),\n.draft = false,\n"
	out, err := ziggyToYaml(fm)
	if err != nil {
		t.Fatalf("ziggyToYaml() error = %v", err)
	}
	for _, want := range []string{"title: My Post", "draft: false", "2024-01-15"} {
		if !strings.Contains(out, want) {
			t.Errorf("ziggyToYaml() = %q, missing %q", out, want)
		}
	}
}
