package officialaccount

import (
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	nethtml "golang.org/x/net/html"
)

type ArticleSaveOptions struct {
	OutputDir       string
	KeepVideo       bool
	SaveContentHTML bool
}

type SavedArticle struct {
	Title       string
	ContentHTML string
}

type ArticleSaveResult struct {
	Title           string
	MarkdownPath    string
	ContentHTMLPath string
}

var (
	contentNoencodeReg = regexp.MustCompile(`content_noencode\s*:\s*(['"])`)
	jsTitleRegs        = []*regexp.Regexp{
		regexp.MustCompile(`var\s+msg_title\s*=\s*(['"])`),
		regexp.MustCompile(`msg_title\s*:\s*(['"])`),
		regexp.MustCompile(`title\s*:\s*(['"])`),
	}
	titleTagReg                   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	activityNameReg               = regexp.MustCompile(`(?is)id=["']activity-name["'][^>]*>(.*?)</[^>]+>`)
	jsContentReg                  = regexp.MustCompile(`(?is)id=["']js_content["'][^>]*>(.*?)</div>`)
	bodyReg                       = regexp.MustCompile(`(?is)<body[^>]*>(.*?)</body>`)
	imgReg                        = regexp.MustCompile(`(?is)<img\b([^>]*)>`)
	imgSrcAttrReg                 = regexp.MustCompile(`(?i)(?:^|\s)src\s*=\s*("([^"]*)"|'([^']*)')`)
	imgDataSrcAttrReg             = regexp.MustCompile(`(?i)(?:^|\s)data-src\s*=\s*("([^"]*)"|'([^']*)')`)
	imgAltAttrReg                 = regexp.MustCompile(`(?i)(?:^|\s)alt\s*=\s*("([^"]*)"|'([^']*)')`)
	videoSnapReg                  = regexp.MustCompile(`(?is)<mp-common-videosnap\b([^>]*)>.*?</mp-common-videosnap>`)
	videoURLAttrReg               = regexp.MustCompile(`(?i)(?:^|\s)data-url\s*=\s*("([^"]*)"|'([^']*)')`)
	preReg                        = regexp.MustCompile(`(?is)<pre\b.*?</pre>`)
	codeContentReg                = regexp.MustCompile(`(?is)<code\b[^>]*>(.*?)</code>`)
	brReg                         = regexp.MustCompile(`(?i)<br\b[^>]*\/?>`)
	tagReg                        = regexp.MustCompile(`(?is)<[^>]+>`)
	highlightSpanReg              = regexp.MustCompile(`(?is)<span\b[^>]*\bleaf\s*=`)
	highlightGapReg               = regexp.MustCompile(`(?is)(?:</span>\s*)+(<span\b[^>]*>)`)
	highlightedCodeSpaceReg       = regexp.MustCompile(` {2,}`)
	highlightedCodeTightSuffixReg = regexp.MustCompile(`\s+([)\].,;:])`)
	highlightedCodeTightPrefixReg = regexp.MustCompile(`([(\[])\s+`)
	highlightedCodeCallReg        = regexp.MustCompile(`([A-Za-z0-9_$])\s+\(`)
	blockquoteReg                 = regexp.MustCompile(`(?is)<blockquote\b[^>]*>(.*?)</blockquote>`)
	codeFenceReg                  = regexp.MustCompile(`(?is)<pre><code>(.*?)</code></pre>`)
	h1Reg                         = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	h2Reg                         = regexp.MustCompile(`(?is)<h2\b[^>]*>(.*?)</h2>`)
	h3Reg                         = regexp.MustCompile(`(?is)<h3\b[^>]*>(.*?)</h3>`)
	h4Reg                         = regexp.MustCompile(`(?is)<h4\b[^>]*>(.*?)</h4>`)
	linkReg                       = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	hrefAttrReg                   = regexp.MustCompile(`(?i)(?:^|\s)href\s*=\s*("([^"]*)"|'([^']*)')`)
	pReg                          = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	liReg                         = regexp.MustCompile(`(?is)<li\b[^>]*>(.*?)</li>`)
	listWrapReg                   = regexp.MustCompile(`(?is)</?(ul|ol)\b[^>]*>`)
	structuralTagReg              = regexp.MustCompile(`(?is)</?(section|figure|span)\b[^>]*>`)
	strongReg                     = regexp.MustCompile(`(?is)<(strong|b)\b[^>]*>(.*?)</(strong|b)>`)
	emReg                         = regexp.MustCompile(`(?is)<(em|i)\b[^>]*>(.*?)</(em|i)>`)
	inlineCodeReg                 = regexp.MustCompile(`(?is)<code\b[^>]*>(.*?)</code>`)
	hrReg                         = regexp.MustCompile(`(?i)<hr\b[^>]*\/?>`)
)

func ExtractArticleFromHTML(source []byte, fallbackPath string) (*SavedArticle, error) {
	htmlText := string(source)
	if content, ok := extractContentNoencode(htmlText); ok {
		return &SavedArticle{Title: extractArticleTitle(htmlText, fallbackPath), ContentHTML: content}, nil
	}
	if match := jsContentReg.FindStringSubmatch(htmlText); len(match) > 1 {
		return &SavedArticle{Title: extractArticleTitle(htmlText, fallbackPath), ContentHTML: match[1]}, nil
	}
	if match := bodyReg.FindStringSubmatch(htmlText); len(match) > 1 {
		return &SavedArticle{Title: extractArticleTitle(htmlText, fallbackPath), ContentHTML: match[1]}, nil
	}
	return nil, fmt.Errorf("no WeChat article content found in %s", fallbackPath)
}

func SaveArticleFromHTML(source []byte, sourceURL string, options ArticleSaveOptions) (*ArticleSaveResult, error) {
	article, err := ExtractArticleFromHTML(source, sourceURL)
	if err != nil {
		return nil, err
	}
	return SaveArticle(article, options)
}

func SaveArticle(article *SavedArticle, options ArticleSaveOptions) (*ArticleSaveResult, error) {
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = "articles"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	baseName := sanitizeArticleFilename(article.Title)
	markdownPath := uniqueArticlePath(filepath.Join(outputDir, baseName+".md"))
	contentHTMLPath := strings.TrimSuffix(markdownPath, filepath.Ext(markdownPath)) + ".content.html"
	preparedHTML := PrepareArticleHTML(article.ContentHTML, options)
	markdown := ArticleHTMLToMarkdown(preparedHTML)

	if err := os.WriteFile(contentHTMLPath, []byte(preparedHTML), 0644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(markdownPath, []byte(markdown), 0644); err != nil {
		return nil, err
	}

	return &ArticleSaveResult{Title: article.Title, MarkdownPath: markdownPath, ContentHTMLPath: contentHTMLPath}, nil
}

func articleSaverMPVideoRequestURL(hostname, pathname, rawQuery string) (string, bool) {
	if hostname != "mpvideo.qpic.cn" {
		return "", false
	}
	url := "https://" + hostname + pathname
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	return url, true
}

func PrepareArticleHTML(contentHTML string, options ArticleSaveOptions) string {
	prepared := imgReg.ReplaceAllStringFunc(contentHTML, func(tag string) string {
		attrs := imgReg.FindStringSubmatch(tag)
		if len(attrs) < 2 || imgSrcAttrReg.MatchString(attrs[1]) {
			return tag
		}
		dataSrc := imgDataSrcAttrReg.FindStringSubmatch(attrs[1])
		if len(dataSrc) < 3 {
			return tag
		}
		return fmt.Sprintf(`<img src="%s"%s>`, attrValue(dataSrc), attrs[1])
	})
	prepared = videoSnapReg.ReplaceAllStringFunc(prepared, func(tag string) string {
		if !options.KeepVideo {
			return ""
		}
		attrs := videoSnapReg.FindStringSubmatch(tag)
		if len(attrs) < 2 {
			return ""
		}
		videoURL := videoURLAttrReg.FindStringSubmatch(attrs[1])
		if len(videoURL) < 3 {
			return ""
		}
		url := attrValue(videoURL)
		return fmt.Sprintf(`<p>Video: <a href="%s">%s</a></p>`, url, url)
	})
	prepared = preReg.ReplaceAllStringFunc(prepared, func(pre string) string {
		codeMatch := codeContentReg.FindStringSubmatch(pre)
		code := pre
		if len(codeMatch) > 1 {
			code = codeMatch[1]
		}
		code = extractPreformattedText(code)
		return "<pre><code>" + stdhtml.EscapeString(strings.TrimRight(code, "\n")) + "</code></pre>"
	})
	return brReg.ReplaceAllString(prepared, "\n")
}

func ArticleHTMLToMarkdown(contentHTML string) string {
	md := PrepareArticleHTML(contentHTML, ArticleSaveOptions{})
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")
	codeBlocks := make([]string, 0)
	md = blockquoteReg.ReplaceAllStringFunc(md, func(block string) string {
		match := blockquoteReg.FindStringSubmatch(block)
		if len(match) < 2 {
			return ""
		}
		quote := strings.TrimSpace(ArticleHTMLToMarkdown(match[1]))
		lines := strings.Split(quote, "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				lines[i] = "> " + line
			}
		}
		return "\n\n" + strings.Join(lines, "\n") + "\n\n"
	})
	md = codeFenceReg.ReplaceAllStringFunc(md, func(block string) string {
		match := codeFenceReg.FindStringSubmatch(block)
		if len(match) < 2 {
			return ""
		}
		code := strings.TrimRight(stdhtml.UnescapeString(match[1]), "\n")
		codeBlocks = append(codeBlocks, "\n\n```\n"+code+"\n```\n\n")
		return articleCodeBlockPlaceholder(len(codeBlocks) - 1)
	})
	md = strongReg.ReplaceAllString(md, "**$2**")
	md = emReg.ReplaceAllString(md, "*$2*")
	md = h1Reg.ReplaceAllStringFunc(md, func(s string) string { return headingMarkdown(s, h1Reg, "#") })
	md = h2Reg.ReplaceAllStringFunc(md, func(s string) string { return headingMarkdown(s, h2Reg, "##") })
	md = h3Reg.ReplaceAllStringFunc(md, func(s string) string { return headingMarkdown(s, h3Reg, "###") })
	md = h4Reg.ReplaceAllStringFunc(md, func(s string) string { return headingMarkdown(s, h4Reg, "####") })
	md = imgReg.ReplaceAllStringFunc(md, func(tag string) string {
		attrs := imgReg.FindStringSubmatch(tag)
		if len(attrs) < 2 {
			return ""
		}
		src := imgSrcAttrReg.FindStringSubmatch(attrs[1])
		if len(src) < 3 {
			return ""
		}
		imageURL := strings.TrimSpace(stdhtml.UnescapeString(attrValue(src)))
		if imageURL == "" {
			return ""
		}
		altText := ""
		if alt := imgAltAttrReg.FindStringSubmatch(attrs[1]); len(alt) > 2 {
			altText = stripArticleTags(attrValue(alt))
		}
		return "\n\n![" + altText + "](" + imageURL + ")\n\n"
	})
	md = linkReg.ReplaceAllStringFunc(md, func(link string) string {
		match := linkReg.FindStringSubmatch(link)
		if len(match) < 3 {
			return stripArticleTags(link)
		}
		href := hrefAttrReg.FindStringSubmatch(match[1])
		label := stripArticleTags(match[2])
		if len(href) < 3 {
			return label
		}
		return "[" + label + "](" + stdhtml.UnescapeString(attrValue(href)) + ")"
	})
	md = pReg.ReplaceAllStringFunc(md, func(p string) string {
		match := pReg.FindStringSubmatch(p)
		if len(match) < 2 {
			return ""
		}
		return "\n\n" + stripArticleTags(match[1]) + "\n\n"
	})
	md = liReg.ReplaceAllStringFunc(md, func(li string) string {
		match := liReg.FindStringSubmatch(li)
		if len(match) < 2 {
			return ""
		}
		return "\n- " + stripArticleTags(match[1])
	})
	md = listWrapReg.ReplaceAllString(md, "\n")
	md = structuralTagReg.ReplaceAllString(md, "")
	md = inlineCodeReg.ReplaceAllStringFunc(md, func(code string) string {
		match := inlineCodeReg.FindStringSubmatch(code)
		if len(match) < 2 {
			return ""
		}
		return "`" + stripArticleTags(match[1]) + "`"
	})
	md = hrReg.ReplaceAllString(md, "\n\n---\n\n")
	md = tagReg.ReplaceAllString(md, "")
	md = strings.ReplaceAll(stdhtml.UnescapeString(md), "\u00a0", " ")
	for i, block := range codeBlocks {
		md = strings.ReplaceAll(md, articleCodeBlockPlaceholder(i), block)
	}
	return normalizeArticleMarkdown(md)
}

func extractPreformattedText(source string) string {
	decodedSource := stdhtml.UnescapeString(source)
	if highlightSpanReg.MatchString(decodedSource) {
		decodedSource = highlightGapReg.ReplaceAllString(decodedSource, articleHighlightGapPlaceholder+`$1`)
		decodedSource = brReg.ReplaceAllString(decodedSource, "\n")
		decodedSource = tagReg.ReplaceAllString(decodedSource, "")
		decodedSource = restoreHighlightedCodeSpacing(decodedSource)
		decodedSource = normalizeHighlightedCodeText(decodedSource)
		return strings.ReplaceAll(decodedSource, "\u00a0", " ")
	}

	text, ok := extractTextFromHTMLFragment(source)
	if !ok {
		text := brReg.ReplaceAllString(source, "\n")
		return strings.ReplaceAll(stdhtml.UnescapeString(text), "\u00a0", " ")
	}
	decoded := stdhtml.UnescapeString(text)
	if highlightSpanReg.MatchString(decoded) {
		decoded = brReg.ReplaceAllString(decoded, "\n")
		text = tagReg.ReplaceAllString(decoded, "")
	} else {
		text = decoded
	}
	return strings.ReplaceAll(text, "\u00a0", " ")
}

func extractTextFromHTMLFragment(source string) (string, bool) {
	fragment, err := nethtml.ParseFragment(strings.NewReader("<div>"+source+"</div>"), &nethtml.Node{
		Type: nethtml.ElementNode,
		Data: "div",
	})
	if err != nil {
		return "", false
	}

	var root *nethtml.Node
	for _, node := range fragment {
		if node.Type == nethtml.ElementNode && node.Data == "div" {
			root = node
			break
		}
	}
	if root == nil {
		return "", false
	}

	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil {
			return
		}
		switch node.Type {
		case nethtml.TextNode:
			b.WriteString(node.Data)
		case nethtml.ElementNode:
			if node.Data == "br" {
				b.WriteByte('\n')
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walk(child)
	}
	return b.String(), true
}

func extractContentNoencode(source string) (string, bool) {
	match := contentNoencodeReg.FindStringSubmatchIndex(source)
	if len(match) < 4 {
		return "", false
	}
	start := match[2]
	literal, err := scanJSStringLiteral(source, start)
	if err != nil {
		return "", false
	}
	decoded, err := decodeJSStringLiteral(literal)
	if err != nil {
		return "", false
	}
	return decoded, true
}

func extractArticleTitle(source string, fallbackPath string) string {
	for _, reg := range jsTitleRegs {
		match := reg.FindStringSubmatchIndex(source)
		if len(match) >= 4 {
			if literal, err := scanJSStringLiteral(source, match[2]); err == nil {
				if decoded, err := decodeJSStringLiteral(literal); err == nil && strings.TrimSpace(decoded) != "" {
					return stripArticleTags(decoded)
				}
			}
		}
	}
	if match := titleTagReg.FindStringSubmatch(source); len(match) > 1 {
		title := strings.TrimSpace(strings.TrimSuffix(stripArticleTags(match[1]), "- 微信公众平台"))
		if title != "" {
			return title
		}
	}
	if match := activityNameReg.FindStringSubmatch(source); len(match) > 1 {
		if title := stripArticleTags(match[1]); title != "" {
			return title
		}
	}
	name := strings.TrimSuffix(filepath.Base(fallbackPath), filepath.Ext(fallbackPath))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "wechat-article"
	}
	return name
}

func scanJSStringLiteral(source string, start int) (string, error) {
	if start < 0 || start >= len(source) || (source[start] != '\'' && source[start] != '"') {
		return "", errors.New("expected JavaScript string literal")
	}
	quote := source[start]
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		if source[i] == quote {
			return source[start : i+1], nil
		}
	}
	return "", errors.New("unterminated JavaScript string literal")
}

func decodeJSStringLiteral(literal string) (string, error) {
	if len(literal) < 2 {
		return "", errors.New("invalid JavaScript string literal")
	}
	quote := literal[0]
	body := literal[1 : len(literal)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if ch != '\\' {
			b.WriteByte(ch)
			continue
		}
		i++
		if i >= len(body) {
			return "", errors.New("invalid trailing escape")
		}
		switch body[i] {
		case 'x':
			if i+2 >= len(body) {
				return "", errors.New("invalid hex escape")
			}
			n, err := strconv.ParseInt(body[i+1:i+3], 16, 32)
			if err != nil {
				return "", err
			}
			b.WriteRune(rune(n))
			i += 2
		case 'u':
			if i+4 >= len(body) {
				return "", errors.New("invalid unicode escape")
			}
			n, err := strconv.ParseInt(body[i+1:i+5], 16, 32)
			if err != nil {
				return "", err
			}
			b.WriteRune(rune(n))
			i += 4
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case '\\':
			b.WriteByte('\\')
		case '\'', '"':
			if body[i] != quote {
				b.WriteByte('\\')
			}
			b.WriteByte(body[i])
		default:
			b.WriteByte(body[i])
		}
	}
	return b.String(), nil
}

func headingMarkdown(source string, reg *regexp.Regexp, prefix string) string {
	match := reg.FindStringSubmatch(source)
	if len(match) < 2 {
		return ""
	}
	return "\n\n" + prefix + " " + stripArticleTags(match[1]) + "\n\n"
}

func attrValue(match []string) string {
	if len(match) > 2 && match[2] != "" {
		return match[2]
	}
	if len(match) > 3 {
		return match[3]
	}
	return ""
}

func articleCodeBlockPlaceholder(index int) string {
	return fmt.Sprintf("ARTICLECODEBLOCK%dPLACEHOLDER", index)
}

const articleHighlightGapPlaceholder = "ARTICLEHIGHLIGHTGAPPLACEHOLDER"

func restoreHighlightedCodeSpacing(source string) string {
	var b strings.Builder
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], articleHighlightGapPlaceholder) {
			prev, hasPrev := previousNonSpaceRune(source[:i])
			next, hasNext := nextNonSpaceRune(source[i+len(articleHighlightGapPlaceholder):])
			if hasPrev && hasNext && shouldInsertHighlightedSpace(prev, next) {
				b.WriteByte(' ')
			}
			i += len(articleHighlightGapPlaceholder)
			continue
		}
		r, size := utf8.DecodeRuneInString(source[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

func normalizeHighlightedCodeText(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		indentLen := len(trimmed) - len(strings.TrimLeft(trimmed, " \t"))
		if indentLen == 1 {
			indentLen = 0
		}
		indent := trimmed[:indentLen]
		body := trimmed[indentLen:]
		if strings.HasPrefix(body, " ") && !strings.HasPrefix(body, "  ") {
			body = strings.TrimLeft(body, " ")
		}
		body = highlightedCodeSpaceReg.ReplaceAllString(body, " ")
		body = highlightedCodeTightSuffixReg.ReplaceAllString(body, `$1`)
		body = highlightedCodeTightPrefixReg.ReplaceAllString(body, `$1`)
		body = highlightedCodeCallReg.ReplaceAllString(body, `$1(`)
		lines[i] = indent + body
	}
	return strings.Join(lines, "\n")
}

func previousNonSpaceRune(source string) (rune, bool) {
	for len(source) > 0 {
		r, size := utf8.DecodeLastRuneInString(source)
		source = source[:len(source)-size]
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func nextNonSpaceRune(source string) (rune, bool) {
	for len(source) > 0 {
		r, size := utf8.DecodeRuneInString(source)
		source = source[size:]
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func shouldInsertHighlightedSpace(prev rune, next rune) bool {
	if unicode.IsSpace(prev) || unicode.IsSpace(next) {
		return false
	}
	if strings.ContainsRune("([{.,:;<>+-*/=%&|!?~", prev) {
		return false
	}
	if strings.ContainsRune(")]},.:;<>+-*/=%&|!?~", next) {
		return false
	}
	return true
}

func stripArticleTags(source string) string {
	text := brReg.ReplaceAllString(source, "\n")
	text = tagReg.ReplaceAllString(text, "")
	text = strings.ReplaceAll(stdhtml.UnescapeString(text), "\u00a0", " ")
	return strings.TrimSpace(text)
}

func normalizeArticleMarkdown(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text := strings.Join(lines, "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text) + "\n"
}

func sanitizeArticleFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "wechat-article"
	}
	var b strings.Builder
	for _, r := range name {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) || !utf8.ValidRune(r) {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	sanitized := strings.Join(strings.Fields(b.String()), " ")
	if sanitized == "" {
		return "wechat-article"
	}
	runes := []rune(sanitized)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return sanitized
}

func uniqueArticlePath(filePath string) string {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return filePath
	}
	dir := filepath.Dir(filePath)
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), ext)
	for i := 2; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
