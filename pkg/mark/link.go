package mark

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"errors"

	"github.com/kovetskiy/mark/pkg/confluence"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type LinkSubstitution struct {
	From string
	To   string
}

type markdownLink struct {
	full     string
	filename string
	hash     string
}

func ResolveRelativeLinks(
	api *confluence.API,
	meta *Meta,
	markdown []byte,
	base string,
) ([]LinkSubstitution, error) {
	matches := parseLinks(string(markdown))

	links := []LinkSubstitution{}
	var partial_fail error = nil
	for _, match := range matches {
		log.Tracef(
			nil,
			"found a relative link: full=%s filename=%s hash=%s",
			match.full,
			match.filename,
			match.hash,
		)

		resolved, err := resolveLink(api, base, match)
		if err != nil {
			return nil, karma.Format(err, "resolve link: %q", match.full)
		}

		if resolved == "" {
			partial_fail = errors.New("resolved relative link partial fails")
			continue
		}

		links = append(links, LinkSubstitution{
			From: match.full,
			To:   resolved,
		})
	}

	return links, partial_fail 
}

func resolveLink(
	api *confluence.API,
	base string,
	link markdownLink,
) (string, error) {
	var result string

	if len(link.filename) > 0 {
		filepath := filepath.Join(base, link.filename)

		stat, err := os.Stat(filepath)
		if err != nil {
			return "", nil
		}

		if stat.IsDir() {
			return "", nil
		}

		linkContents, err := ioutil.ReadFile(filepath)
		if err != nil {
			return "", karma.Format(err, "read file: %s", filepath)
		}

		linkContents = bytes.ReplaceAll(
			linkContents,
			[]byte("\r\n"),
			[]byte("\n"),
		)

		// This helps to determine if found link points to file that's
		// not markdown or have mark required metadata
		linkMeta, _, err := ExtractMeta(linkContents)
		if err != nil {
			log.Errorf(
				err,
				"unable to extract metadata from %q; ignoring the relative link",
				filepath,
			)

			return "", nil
		}

		if linkMeta == nil {
			return "", nil
		}

		result, err = getConfluenceLink(api, linkMeta.Space, linkMeta.Title)
		if err != nil {
			return "", karma.Format(
				err,
				"find confluence page: %s / %s / %s",
				filepath,
				linkMeta.Space,
				linkMeta.Title,
			)
		}

		if result == "" {
			return "", nil
		}
	}

	if len(link.hash) > 0 {
		result = result + "#" + link.hash
	}

	return result, nil
}

func SubstituteLinks(markdown []byte, links []LinkSubstitution) []byte {
	for _, link := range links {
		if link.From == link.To {
			continue
		}

		log.Tracef(nil, "substitute link: %q -> %q", link.From, link.To)

		markdown = bytes.ReplaceAll(
			markdown,
			[]byte(fmt.Sprintf("](%s)", link.From)),
			[]byte(fmt.Sprintf("](%s)", link.To)),
		)
	}

	return markdown
}

func parseLinks(markdown string) []markdownLink {
	re := regexp.MustCompile("\\[[^\\]]+\\]\\((([^\\)#]+)?#?([^\\)]+)?)\\)")
	matches := re.FindAllStringSubmatch(markdown, -1)

	links := make([]markdownLink, len(matches))
	for i, match := range matches {
		links[i] = markdownLink{
			full:     match[1],
			filename: match[2],
			hash:     match[3],
		}
	}

	return links
}

// getConfluenceLink build (to be) link for Conflunce, and tries to verify from
// API if there's real link available
func getConfluenceLink(
	api *confluence.API,
	space, title string,
) (string, error) {
	// link := fmt.Sprintf(
	// 	"%s/display/%s/%s",
	// 	api.BaseURL,
	// 	space,
	// 	url.QueryEscape(title),
	// )
	var link string

	page, err := api.FindPage(space, title, "page")
	if err != nil {
		return "", karma.Format(err, "api: find page")
	}

	if page != nil {
		// Needs baseURL, as REST api response URL doesn't contain subpath ir
		// confluence is server from that
		link = api.BaseURL + page.Links.Full
	} else {
		link = ""
	}

	return link, nil
}
