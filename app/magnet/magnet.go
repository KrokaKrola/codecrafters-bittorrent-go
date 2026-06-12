package magnet

import (
	"fmt"
	"net/url"
	"strings"
)

type Magnet struct {
	Xt         string // raw xt param
	InfoHash   string // info hash from xt param
	FileName   string // dn param
	TrackerUrl string // tr
}

func NewMagnet(link string) (*Magnet, error) {
	if !strings.HasPrefix(link, "magnet:?") {
		return nil, fmt.Errorf("magnet link must start with magnet:?")
	}

	queryValues, err := url.ParseQuery(link[8:])
	if err != nil {
		return nil, fmt.Errorf("invalid magnet link: %s", err.Error())
	}

	xt := queryValues.Get("xt")
	if xt == "" {
		return nil, fmt.Errorf("xt parameter is required in magnet links")
	}

	if !strings.HasPrefix(xt, "urn:btih:") {
		return nil, fmt.Errorf("xt parameter has invalid format. valid format is urn:btih:<hash>")
	}

	return &Magnet{
		Xt:         xt,
		FileName:   queryValues.Get("dn"),
		TrackerUrl: queryValues.Get("tr"),
		InfoHash:   xt[9:],
	}, nil
}
