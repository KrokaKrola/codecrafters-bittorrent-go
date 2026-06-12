package magnet

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/app/peer"
)

type Magnet struct {
	InfoHash   []byte // info hash from xt param
	FileName   string // dn param
	TrackerUrl string // tr
	Peers      []*peer.Peer
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

	decodedHash, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash value")
	}

	mg := &Magnet{
		FileName:   queryValues.Get("dn"),
		TrackerUrl: queryValues.Get("tr"),
		InfoHash:   decodedHash,
	}

	if err := mg.enrichWithPeers(); err != nil {
		return nil, err
	}

	return mg, nil
}

func (mg *Magnet) enrichWithPeers() error {
	// tracker requires left value to be greater than zero, but we dont know file size in advance, so 999 is a workaround
	peers, err := peer.GetPeers(mg.TrackerUrl, string(mg.InfoHash), 999)
	if err != nil {
		return err
	}

	mg.Peers = peers
	return nil
}
