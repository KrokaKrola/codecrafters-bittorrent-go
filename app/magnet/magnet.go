package magnet

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/app/peer"
)

type Magnet struct {
	InfoHash    []byte // info hash from xt param
	FileName    string // dn param
	TrackerUrl  string // tr
	Peers       []*peer.Peer
	PieceLength int
	Pieces      string
	Length      int
	PieceParts  [][]byte
}

const infoBlockSize = 16384

func (mg *Magnet) GetPiece(conn net.Conn, pieceIndex int) ([]byte, error) {
	actualPieceLength := mg.PieceLength
	if pieceIndex == len(mg.PieceParts)-1 {
		actualPieceLength = mg.Length - (pieceIndex * mg.PieceLength)
	}

	var blocks [][]byte
	for begin := uint32(0); begin < uint32(actualPieceLength); begin += infoBlockSize {
		length := min(infoBlockSize, uint32(actualPieceLength)-begin)

		requestMsg := []byte{}
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, 13)...)
		requestMsg = append(requestMsg, byte(6))
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, uint32(pieceIndex))...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, begin)...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, length)...)

		if _, err := conn.Write(requestMsg); err != nil {
			return nil, fmt.Errorf("error writing request msg to peer")
		}

		var block []byte
		for {
			lenBuf := make([]byte, 4)
			if _, err := io.ReadFull(conn, lenBuf); err != nil {
				return nil, fmt.Errorf("error reading msg size for block request")
			}
			msgLen := binary.BigEndian.Uint32(lenBuf)
			if msgLen == 0 {
				continue
			}
			pieceMsg := make([]byte, msgLen)
			if _, err := io.ReadFull(conn, pieceMsg); err != nil {
				return nil, fmt.Errorf("error reading block msg from peer")
			}
			if pieceMsg[0] != 7 {
				continue
			}
			if msgLen < 9 {
				return nil, fmt.Errorf("piece message too short: %d", msgLen)
			}
			block = pieceMsg[9:]
			break
		}
		blocks = append(blocks, block)
	}

	return slices.Concat(blocks...), nil
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
