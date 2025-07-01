# epub2html

`epub2html` is a dead-simple command-line tool written in Go to convert EPUB files into a single, raw HTML file. It extracts content from the EPUB's chapters and combines them sequentially.

## Features

- Parses EPUB container and package files.
- Reads content documents based on the EPUB spine.
- Extracts HTML content from the `<body>` of each content document.
- Combines extracted HTML into a single output file.
- Embeds images directly into the HTML file using base64 encoding.
- Strips scripts, styles, and other non-content elements to produce "raw" HTML.
- Preserves basic HTML structure and attributes of content tags.

## Prerequisites

- Go

## Installation & Building

1. Clone the repository (or download the source code).
2. Navigate to the project directory:
   ```bash
   cd path/to/epub2html
   ```
3. Build the executable:
   ```bash
   go build
   ```
   This will create an `epub2html` executable in the current directory.

## Usage

```bash
./epub2html <path_to_epub_file> [path_to_output_html_file]
```

**Arguments:**

- `path_to_epub_file` (required): Path to the input EPUB file.
- `path_to_output_html_file` (optional): Path to the output HTML file. Defaults to `output.html`.

**Example:**

```bash
./epub2html mybook.epub mybook_converted.html
```

## Limitations

- **Raw HTML Output:** The primary goal is to extract textual content with basic structure. Complex styling, scripts, and other embedded media (like videos) are removed.
- **CSS and Styling:** All CSS styles are stripped. The output HTML will be unstyled.
- **Font Embedding:** Embedded fonts are not handled.
