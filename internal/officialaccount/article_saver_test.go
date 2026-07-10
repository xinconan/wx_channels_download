package officialaccount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArticleFromHTML(t *testing.T) {
	html := `<script>
var msg_title = 'DeepAgents：开箱即用';
var item = {
  content_noencode: '\x3cp\x3e正文第一段\x3c/p\x3e'
};
</script>`

	article, err := ExtractArticleFromHTML([]byte(html), "sample.html")
	if err != nil {
		t.Fatalf("ExtractArticleFromHTML returned error: %v", err)
	}

	if article.Title != "DeepAgents：开箱即用" {
		t.Fatalf("Title = %q, want DeepAgents：开箱即用", article.Title)
	}
	if article.ContentHTML != "<p>正文第一段</p>" {
		t.Fatalf("ContentHTML = %q, want article content", article.ContentHTML)
	}
}

func TestPrepareArticleHTMLFixesImagesAndRemovesVideos(t *testing.T) {
	html := PrepareArticleHTML(`<p>看图</p>
<img data-src="https://mmbiz.qpic.cn/test/640?wx_fmt=png" alt="demo">
<mp-common-videosnap data-url="https://findermp.video.qq.com/encrypted"></mp-common-videosnap>`, ArticleSaveOptions{})

	if !strings.Contains(html, `<img src="https://mmbiz.qpic.cn/test/640?wx_fmt=png"`) {
		t.Fatalf("prepared HTML does not add src from data-src:\n%s", html)
	}
	if strings.Contains(html, "findermp.video.qq.com") {
		t.Fatalf("prepared HTML should remove encrypted video links:\n%s", html)
	}
}

func TestArticleHTMLToMarkdown(t *testing.T) {
	markdown := ArticleHTMLToMarkdown(`<h2>小标题</h2>
<p>第一段<br>第二行</p>
<pre><code>npm init -y<br>node index.js</code></pre>
<blockquote><p>引用内容</p></blockquote>
<img src="https://mmbiz.qpic.cn/test/640?wx_fmt=png" alt="demo">`)

	for _, want := range []string{
		"## 小标题",
		"第一段\n第二行",
		"```\nnpm init -y\nnode index.js\n```",
		"> 引用内容",
		"![demo](https://mmbiz.qpic.cn/test/640?wx_fmt=png)",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestSaveArticleFromHTMLWritesMarkdownAndContentHTML(t *testing.T) {
	outputDir := t.TempDir()
	html := `<script>
var msg_title = '可保存的文章';
var item = {
  content_noencode: '\x3cp\x3e正文\x3c/p\x3e\x3cimg data-src=\x22https://mmbiz.qpic.cn/a.png\x22\x3e\x3cmp-common-videosnap data-url=\x22https://findermp.video.qq.com/encrypted\x22\x3e\x3c/mp-common-videosnap\x3e'
};
</script>`

	result, err := SaveArticleFromHTML([]byte(html), "https://mp.weixin.qq.com/s?__biz=test", ArticleSaveOptions{
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("SaveArticleFromHTML returned error: %v", err)
	}

	if filepath.Base(result.MarkdownPath) != "可保存的文章.md" {
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
	if !strings.Contains(got, "正文") {
		t.Fatalf("markdown missing article body:\n%s", got)
	}
	if !strings.Contains(got, "mmbiz.qpic.cn/a.png") {
		t.Fatalf("markdown missing image link:\n%s", got)
	}
	if strings.Contains(got, "findermp.video.qq.com") {
		t.Fatalf("markdown should remove encrypted video link:\n%s", got)
	}
}
