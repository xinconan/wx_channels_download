package officialaccount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArticleFromHTML(t *testing.T) {
	source := `<script>
var msg_title = 'Sample Article';
var item = {
  content_noencode: '\x3cp\x3ehello world\x3c/p\x3e'
};
</script>`

	article, err := ExtractArticleFromHTML([]byte(source), "sample.html")
	if err != nil {
		t.Fatalf("ExtractArticleFromHTML returned error: %v", err)
	}

	if article.Title != "Sample Article" {
		t.Fatalf("Title = %q, want %q", article.Title, "Sample Article")
	}
	if article.ContentHTML != "<p>hello world</p>" {
		t.Fatalf("ContentHTML = %q, want %q", article.ContentHTML, "<p>hello world</p>")
	}
}

func TestPrepareArticleHTMLFixesImagesAndRemovesVideos(t *testing.T) {
	html := PrepareArticleHTML(`<p>look</p>
<img data-src="https://mmbiz.qpic.cn/test/640?wx_fmt=png" alt="demo">
<mp-common-videosnap data-url="https://findermp.video.qq.com/encrypted"></mp-common-videosnap>`, ArticleSaveOptions{})

	if !strings.Contains(html, `<img src="https://mmbiz.qpic.cn/test/640?wx_fmt=png"`) {
		t.Fatalf("prepared HTML does not add src from data-src:\n%s", html)
	}
	if strings.Contains(html, "findermp.video.qq.com") {
		t.Fatalf("prepared HTML should remove encrypted video links:\n%s", html)
	}
}

func TestArticleSaverMPVideoRequestURL(t *testing.T) {
	got, ok := articleSaverMPVideoRequestURL("mpvideo.qpic.cn", "/0bc3x2p4pnv", "dis_k=token&__biz=biz")
	if !ok {
		t.Fatal("mpvideo request should be recognized")
	}
	want := "https://mpvideo.qpic.cn/0bc3x2p4pnv?dis_k=token&__biz=biz"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}

	if _, ok := articleSaverMPVideoRequestURL("mmbiz.qpic.cn", "/image", "wx_fmt=png"); ok {
		t.Fatal("non-mpvideo request should not be recognized")
	}
}

func TestArticleHTMLToMarkdown(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<h2>Heading</h2>
<p>first line<br>second line</p>
<p><strong>strong text</strong> and <b>bold text</b></p>
<pre><code>npm init -y<br>node index.js</code></pre>
<blockquote><p>quoted text</p></blockquote>
<img src="https://mmbiz.qpic.cn/test/640?wx_fmt=png" alt="demo">`)

	for _, want := range []string{
		"## Heading",
		"first line\nsecond line",
		"```\nnpm init -y\nnode index.js\n```",
		"> quoted text",
		"![demo](https://mmbiz.qpic.cn/test/640?wx_fmt=png)",
		"**strong text** and **bold text**",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestArticleHTMLToMarkdownPreservesCodeComparisons(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<p>before</p>
<pre><code>if (messages.length &lt;= prevCount) {<br>  handler(() =&gt; "ok")<br>}</code></pre>
<p>after</p>`)

	for _, want := range []string{
		"```\nif (messages.length <= prevCount) {\n  handler(() => \"ok\")\n}\n```",
		"\nafter\n",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}

	if strings.Contains(markdown, "prevCount) {\nafter") {
		t.Fatalf("paragraph after code block was merged into code fence:\n%s", markdown)
	}
}

func TestArticleHTMLToMarkdownDecodesHighlightedCodeBlocks(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<p>demo</p>
<pre><code>&lt;span leaf=&#34;&#34;&gt;mkdir mem0-test&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt;<br>&lt;/span&gt;&lt;span style=&#34;color: #e6c07b;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;cd&lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt; mem0-test&lt;/span&gt;<br>&lt;span style=&#34;color: #c678dd;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;import&lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt; { MemoryClient } &lt;/span&gt;&lt;span style=&#34;color: #c678dd;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;from&lt;/span&gt;&lt;/span&gt;&lt;span style=&#34;color: #98c379;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;&#39;mem0ai&#39;&lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt;;&lt;/span&gt;<br>&lt;span style=&#34;color: #c678dd;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;async&lt;/span&gt;&lt;/span&gt;&lt;span style=&#34;line-height: 26px;&#34;&gt;&lt;span style=&#34;color: #c678dd;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;function&lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt; &lt;/span&gt;&lt;span style=&#34;color: #61aeee;line-height: 26px;&#34;&gt;&lt;span leaf=&#34;&#34;&gt;main&lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt;() &lt;/span&gt;&lt;/span&gt;&lt;span leaf=&#34;&#34;&gt;{&lt;/span&gt;</code></pre>`)

	for _, want := range []string{
		"```\nmkdir mem0-test\ncd mem0-test",
		"import { MemoryClient } from 'mem0ai';",
		"async function main() {",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing decoded highlighted code block %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "<span") {
		t.Fatalf("markdown should not keep span markup inside code fences:\n%s", markdown)
	}
}

func TestArticleHTMLToMarkdownConvertsTables(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<p>before</p>
<table style="display: table;text-align: left;"><thead><tr><th style="color: rgb(63, 63, 63);"><section><span leaf="">type</span></section></th><th style="color: rgb(63, 63, 63);"><section><span leaf="">含义</span></section></th></tr></thead><tbody><tr><td><section><span leaf="">PERSON</span></section></td><td><section><span leaf="">人、角色</span></section></td></tr><tr><td><section><span leaf="">ORGANIZATION</span></section></td><td><section><span leaf="">组织、部门</span></section></td></tr></tbody></table>
<p>after</p>`)

	for _, want := range []string{
		"| type | 含义 |",
		"| --- | --- |",
		"| PERSON | 人、角色 |",
		"| ORGANIZATION | 组织、部门 |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing table row %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "<table") || strings.Contains(markdown, "<td") || strings.Contains(markdown, "<th") {
		t.Fatalf("markdown should not keep table markup:\n%s", markdown)
	}
}

func TestArticleHTMLToMarkdownEscapesTablePipesAndInlineCode(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<table><thead><tr><th>边类型</th><th>方向</th></tr></thead><tbody><tr><td><section><span leaf=""><code>HAS_CHUNK</code></span></section></td><td><section><span leaf="">Document → Chunk</span></section></td></tr><tr><td><section><span leaf="">a|b</span></section></td><td><section><span leaf="">c</span></section></td></tr></tbody></table>`)

	for _, want := range []string{
		"| 边类型 | 方向 |",
		"| `HAS_CHUNK` | Document → Chunk |",
		"| a\\|b | c |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing table cell %q:\n%s", want, markdown)
		}
	}
}

func TestArticleHTMLToMarkdownSkipsEmptyImages(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<p>before</p>
<img src="" alt="">
<img data-src="" alt="empty-data-src">
<img src="https://mmbiz.qpic.cn/test/640?wx_fmt=png" alt="demo">
<p>after</p>`)

	if strings.Contains(markdown, "![]()") {
		t.Fatalf("markdown should not contain empty image references:\n%s", markdown)
	}
	if strings.Contains(markdown, "![empty-data-src]()") {
		t.Fatalf("markdown should not contain empty image references from empty data-src:\n%s", markdown)
	}
	if !strings.Contains(markdown, "![demo](https://mmbiz.qpic.cn/test/640?wx_fmt=png)") {
		t.Fatalf("markdown should keep non-empty image references:\n%s", markdown)
	}
}

func TestSaveArticleFromHTMLWritesMarkdownAndContentHTML(t *testing.T) {
	outputDir := t.TempDir()
	source := `<script>
var msg_title = 'Saved Article';
var item = {
  content_noencode: '\x3cp\x3ebody text\x3c/p\x3e\x3cimg data-src=\x22https://mmbiz.qpic.cn/a.png\x22\x3e\x3cmp-common-videosnap data-url=\x22https://findermp.video.qq.com/encrypted\x22\x3e\x3c/mp-common-videosnap\x3e'
};
</script>`

	result, err := SaveArticleFromHTML([]byte(source), "https://mp.weixin.qq.com/s?__biz=test", ArticleSaveOptions{
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("SaveArticleFromHTML returned error: %v", err)
	}

	if filepath.Base(result.MarkdownPath) != "Saved Article.md" {
		t.Fatalf("markdown filename = %q, want title-based filename", filepath.Base(result.MarkdownPath))
	}
	if _, err := os.Stat(result.ContentHTMLPath); err != nil {
		t.Fatalf("content HTML was not written: %v", err)
	}

	markdown, err := os.ReadFile(result.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}

	got := string(markdown)
	if !strings.Contains(got, "body text") {
		t.Fatalf("markdown missing article body:\n%s", got)
	}
	if !strings.Contains(got, "mmbiz.qpic.cn/a.png") {
		t.Fatalf("markdown missing image link:\n%s", got)
	}
	if strings.Contains(got, "findermp.video.qq.com") {
		t.Fatalf("markdown should remove encrypted video link:\n%s", got)
	}
}
