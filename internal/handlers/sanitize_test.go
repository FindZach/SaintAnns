package handlers

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLBlocksScript(t *testing.T) {
	attacks := []string{
		`<scr<script>x</script>ipt>alert(1)</script>`,
		`<img/onerror=alert(1) src=x>`,
		`<details open ontoggle=alert(1)>x</details>`,
		`<svg onload=alert(1)><animate onbegin=alert(1)/></svg>`,
		`<a href="JaVaScRiPt:alert(1)">x</a>`,
		`<a href="&#106;avascript:alert(1)">x</a>`,
		`<iframe srcdoc="<script>alert(1)</script>"></iframe>`,
		`<object data="javascript:alert(1)"></object>`,
		`<base href="https://evil.example/">`,
		`<meta http-equiv="refresh" content="0;url=https://evil.example">`,
		`<form action="https://evil.example"><input name=x></form>`,
		`<p style="background:url(https://evil.example/x.png)">x</p>`,
		`<p style="width:expression(alert(1))">x</p>`,
	}
	for _, in := range attacks {
		out := strings.ToLower(sanitizeHTML(in))
		for _, bad := range []string{"<script", "onerror", "ontoggle", "onload", "onbegin", "javascript:", "<iframe", "srcdoc", "<object", "<base", "<meta", "<form", "<input", "evil.example", "expression"} {
			if strings.Contains(out, bad) {
				t.Errorf("sanitizeHTML(%q) = %q, still contains %q", in, out, bad)
			}
		}
	}
}

func TestSanitizeHTMLKeepsPageContent(t *testing.T) {
	in := `<div class="parish-history-hero" style="background:linear-gradient(135deg,rgba(15,30,48,0.78) 0%,rgba(30,58,95,0.72) 100%),url('/static/images/churches/st-ann-facade.jpg') center/cover no-repeat;color:#faf6ef;padding:4rem 2rem;border-radius:6px;text-align:center"><h2 style="font-family:'Cormorant Garamond',serif;font-weight:500">A History of Faith</h2></div>` +
		`<figure style="margin:1.5rem 0 2rem;background:#1e3a5f"><img src="/static/images/churches/st-ann-exterior.jpg" alt="St. Ann's" loading="lazy" style="width:100%;object-fit:cover"><figcaption style="font-style:italic">Caption</figcaption></figure>` +
		`<p>Call <a href="tel:6073245811">607-324-5811</a> or <a href="https://catholiccourier.com/" target="_blank" rel="noopener">the Courier</a>.</p>` +
		`<table><tbody><tr><td>Mass</td></tr></tbody></table><ul><li><strong>One</strong></li></ul>`
	out := sanitizeHTML(in)
	for _, want := range []string{
		`url(&#39;/static/images/churches/st-ann-facade.jpg&#39;)`,
		`class="parish-history-hero"`,
		`<figure`, `<figcaption`, `object-fit`,
		`src="/static/images/churches/st-ann-exterior.jpg"`,
		`href="tel:6073245811"`, `target="_blank"`, `<table>`, `<strong>One</strong>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sanitizeHTML dropped %q\nout: %s", want, out)
		}
	}
	if strings.Contains(out, "nofollow") {
		t.Errorf("unexpected nofollow added: %s", out)
	}
}

func TestSanitizeHTMLDecodesWordPressEmail(t *testing.T) {
	in := `<p><span id="eeb-320832-35642"></span><script type="text/javascript">document.getElementById("eeb-320832-35642").innerHTML = decodeURIComponent("%64%61%76%69%64%2e%70%72%65%74%65%40%64%6f%72%2e%6f%72%67");</script><noscript>*protected email*</noscript></p>`
	out := sanitizeHTML(in)
	if !strings.Contains(out, `href="mailto:david.prete@dor.org"`) {
		t.Errorf("email not decoded: %s", out)
	}
	if strings.Contains(out, "protected email") || strings.Contains(out, "decodeURIComponent") {
		t.Errorf("encoder leftovers remain: %s", out)
	}
}

func TestSanitizeEmbedHTMLAllowsKnownIframes(t *testing.T) {
	ok := sanitizeEmbedHTML(`<iframe src="https://www.youtube.com/embed/abc" width="560" height="315" allowfullscreen></iframe>`)
	if !strings.Contains(ok, `src="https://www.youtube.com/embed/abc"`) {
		t.Errorf("youtube iframe dropped: %s", ok)
	}
	bad := sanitizeEmbedHTML(`<iframe src="https://evil.example/x"></iframe>`)
	if strings.Contains(bad, "evil.example") {
		t.Errorf("unknown iframe host kept: %s", bad)
	}
}

func TestSafeLinkURL(t *testing.T) {
	good := []string{"https://example.com", "http://x.org/a", "mailto:a@b.org", "tel:6075551212", "/contact/", "#top", "contact/"}
	for _, s := range good {
		if _, ok := safeLinkURL(s); !ok {
			t.Errorf("safeLinkURL(%q) rejected", s)
		}
	}
	bad := []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", " javascript:alert(1)", "data:text/html,x", "vbscript:x", "//evil.example", "/\\evil.example", "java\tscript:alert(1)"}
	for _, s := range bad {
		if _, ok := safeLinkURL(s); ok {
			t.Errorf("safeLinkURL(%q) accepted", s)
		}
	}
}
