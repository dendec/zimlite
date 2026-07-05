# # Comprehensive Markdown Test Document (H1)

This is an extensive markdown file designed to test all features supported by the parser and the SDL renderer.

## Block Elements (H2)

### Paragraphs & Formatting (H3)

Here is a standard paragraph with **strong (bold) text**, *emphasized (italic) text*, and `inline code spans`.
You can also combine formatting like ***bold and italic*** or `code inside **bold** text`.

This paragraph is separated from the previous one. Soft line breaks (like this
one) are treated as spaces in the rendering, but hard line breaks (two spaces at the end of a line)  
should force a new line.

### Lists (Ordered & Unordered)

#### Unordered List:
- Item 1
- Item 2 with **bold text**
- Item 3
  - Nested Item 3a
  - Nested Item 3b
- Item 4

#### Ordered List:
1. First item
2. Second item
3. Third item
   1. Sub-item A
   2. Sub-item B

---

## Technical Elements (H2)

### Fenced Code Blocks

Here is a Go syntax code block:
```go
package main

import "fmt"

func main() {
    // Print a greeting
    fmt.Println("Hello, Kiwix-SDL!")
}
```

And a simple plain text code block:
```
This is a preformatted
plain text block
with multiple lines.
```

### Links & Images

We support links and image alternative texts:
- Link to Google: [Google Search](https://google.com)
- Alt text for image: ![Google Logo](https://www.google.com/images/branding/googlelogo/1x/googlelogo_color_272x92dp.png)
- Local Image: ![Test Image](test_image.png)
- Local link: [Back to Heading](#comprehensive-markdown-test-document-h1)

### Thematic Break

Below is a thematic break (horizontal rule):

---

Thank you for testing the renderer!
