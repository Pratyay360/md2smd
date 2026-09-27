package smd

import (
	"strings"
	"testing"
)

func TestMdToSmd(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "image becomes a directive",
			in:   "![alt](https://example.com/a.png)\n",
			want: "[alt]($image.url(\"https://example.com/a.png\"))\n",
		},
		{
			name: "image title is carried as alt",
			in:   "![pic](a.png \"The Alt\")\n",
			want: "[pic]($image.asset(\"a.png\").alt(\"The Alt\"))\n",
		},
		{
			name: "root-relative image uses siteAsset",
			in:   "![](/img/a.png)\n",
			want: "[]($image.siteAsset(\"/img/a.png\"))\n",
		},
		{
			name: "relative image uses asset",
			in:   "![x](img/a.png)\n",
			want: "[x]($image.asset(\"img/a.png\"))\n",
		},
		{
			name: "external link",
			in:   "[t](https://x.com)\n",
			want: "[t]($link.url(\"https://x.com\").new(true))\n",
		},
		{
			name: "anchor link",
			in:   "[t](#a)\n",
			want: "[t]($link.ref(\"a\"))\n",
		},
		{
			name: "sub link",
			in:   "[t](./s)\n",
			want: "[t]($link.sub(\"s\"))\n",
		},
		{
			name: "code blocks are left alone",
			in:   "```\n![a](b.png)\n```\n",
			want: "```\n![a](b.png)\n```\n",
		},
		{
			name: "frontmatter becomes ziggy",
			in:   "---\ntitle: Hi\n---\n\nbody\n",
			want: "---\n.title = \"Hi\",\n---\n\nbody\n",
		},
		{
			name: "asciinema badge becomes player embed",
			in:   "[![asciicast](https://asciinema.org/a/abc.svg)](https://asciinema.org/a/abc)\n",
			want: "```=html\n<script src=\"https://asciinema.org/a/abc.js\" id=\"asciicast-abc\" async></script>\n```\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MdToSmd(tt.in)
			if err != nil {
				t.Fatalf("MdToSmd() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("MdToSmd()\n got = %q\nwant = %q", got, tt.want)
			}
		})
	}
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
	tests := []string{
		"![alt](https://example.com/a.png)\n",
		"[t](https://x.com)\n",
		"[t](#a)\n",
		"![](/img/a.png)\n",
		"```\n![a](b.png)\n```\n",
		"[![asciicast](https://asciinema.org/a/abc.svg)](https://asciinema.org/a/abc)\n",
		"![pic](a.png \"The Alt\")\n",
		"[t](./s)\n",
	}
	for _, in := range tests {
		smd, err := MdToSmd(in)
		if err != nil {
			t.Fatalf("MdToSmd(%q) error = %v", in, err)
		}
		got, err := SmdToMd(smd)
		if err != nil {
			t.Fatalf("SmdToMd(%q) error = %v", smd, err)
		}
		if got != in {
			t.Errorf("round trip\n  in:   %q\n  smd:  %q\n  back: %q", in, smd, got)
		}
	}
}

// TestLossyRoundTrip pins down conversions that are deliberately not lossless.
// SuperMD has no representation for a bare linked image's caption distinct from
// the inner image alt, and Ziggy frontmatter re-serialisation normalises the
// blank line after the closing fence.
func TestLossyRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "linked image caption becomes the inner alt",
			in:   "[![cap](https://x.com/i.png)](https://y.com)\n",
			want: "[![cap](https://x.com/i.png \"cap\")](https://y.com)\n",
		},
		{
			name: "frontmatter gains a trailing blank line",
			in:   "---\ntitle: Hi\n---\n\n# H\n",
			want: "---\ntitle: Hi\n---\n\n\n# H\n",
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
			if got != tt.want {
				t.Errorf("round trip\n  in:   %q\n  smd:  %q\n  back: %q\n  want: %q", tt.in, smd, got, tt.want)
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
