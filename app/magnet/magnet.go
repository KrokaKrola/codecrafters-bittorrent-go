package magnet

import (
	"fmt"
	"net/url"
	"strings"
)

type Magnet struct {
	InfoHash   string // info hash from xt param
	FileName   string // dn param
	TrackerUrl string // tr
}

func NewMagnet(link string) (*Magnet, error) {
	link, ok := strings.CutPrefix(link, "magnet:?")
	if !ok {
		return nil, fmt.Errorf("magnet link must start with magnet:?")
	}

	queryValues, err := url.ParseQuery(link)
	if err != nil {
		return nil, fmt.Errorf("invalid magnet link: %s", err.Error())
	}

	xt := queryValues.Get("xt")
	if xt == "" {
		return nil, fmt.Errorf("xt parameter is required in magnet links")
	}

	hash, ok := strings.CutPrefix(xt, "urn:btih:")
	if !ok {
		return nil, fmt.Errorf("xt parameter has invalid format. valid format is urn:btih:<hash>")
	}

	return &Magnet{
		FileName:   queryValues.Get("dn"),
		TrackerUrl: queryValues.Get("tr"),
		InfoHash:   hash,
	}, nil
}
