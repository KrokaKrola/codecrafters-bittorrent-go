package torrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"slices"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/app/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/app/utils/stringutil"
)

type fileInfo struct {
	Length      int
	PieceLength int
	pieces      string
	PiecesParts [][]byte
}

type TorrentFile struct {
	Announce    string
	ShaInfoHash string
	infoRaw     bencode.Dictionary
	Info        *fileInfo
	Peers       []*peer.Peer
}

func NewTorrentFile(fileName string) (*TorrentFile, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to read file: %s", err.Error())
	}

	dec := bencode.Decoder{
		Data: data,
	}

	var dict bencode.Dictionary

	if res, ok := dec.Decode().(bencode.Dictionary); ok {
		dict = res
	} else {
		return nil, fmt.Errorf("Invalid file content")
	}

	announce, ok := bencode.FindElementInDictionary[string](dict, "announce")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, announce field was not found")
	}

	info, ok := bencode.FindElementInDictionary[bencode.Dictionary](dict, "info")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, info field was not found")
	}

	encodedInfo, err := bencode.Encode(info)
	if err != nil {
		fmt.Printf("Error while trying to encode info content: %s", err.Error())
		os.Exit(1)
	}

	hash, err := stringutil.CreateSha1HashFromString(encodedInfo, false)
	if err != nil {
		fmt.Println("Error while trying to create hash from info content")
		os.Exit(1)
	}

	infoLength, ok := bencode.FindElementInDictionary[int](info, "length")
	if !ok {
		fmt.Println("Length field in info is not found")
		os.Exit(1)
	}

	pieceLength, ok := bencode.FindElementInDictionary[int](info, "piece length")
	if !ok {
		fmt.Println("Piece length filed in info is not found")
		os.Exit(1)
	}

	pieces, ok := bencode.FindElementInDictionary[string](info, "pieces")
	if !ok {
		fmt.Println("Pieces field is not found")
		os.Exit(1)
	}

	var piecesParts [][]byte

	for i := 0; i < len(pieces); i += 20 {
		piecesParts = append(piecesParts, []byte(pieces[i:i+20]))
	}

	return &TorrentFile{
		Announce:    announce,
		infoRaw:     info,
		ShaInfoHash: hash,
		Info: &fileInfo{
			Length:      infoLength,
			PieceLength: pieceLength,
			pieces:      pieces,
			PiecesParts: piecesParts,
		},
	}, nil
}

func (btFile *TorrentFile) EnrichWithPeers() error {
	peers, err := peer.GetPeers(btFile.Announce, btFile.ShaInfoHash, btFile.Info.Length)
	if err != nil {
		return err
	}

	btFile.Peers = peers

	return nil
}

const infoBlockSize = 16384 // 16 kiB (16 * 1024 bytes)

func (btFile *TorrentFile) GetPiece(peerConn net.Conn, pieceIndex int) ([]byte, error) {
	blocks := [][]byte{}

	// last piece may be smaller than pieceLength if file size is not a multiple of pieceLength
	actualPieceLength := btFile.Info.PieceLength
	if pieceIndex == len(btFile.Info.PiecesParts)-1 {
		actualPieceLength = btFile.Info.Length - (pieceIndex * btFile.Info.PieceLength)
	}

	for begin := uint32(0); begin < uint32(actualPieceLength); begin += infoBlockSize {
		length := min(infoBlockSize, uint32(actualPieceLength)-begin)

		requestMsg := []byte{}

		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, 13)...)
		requestMsg = append(requestMsg, byte(6))
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, uint32(pieceIndex))...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, begin)...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, length)...)

		if _, err := peerConn.Write(requestMsg); err != nil {
			return nil, fmt.Errorf("error writing request msg to peer")
		}

		pieceMsgBuffSize := make([]byte, 4)
		if _, err := io.ReadFull(peerConn, pieceMsgBuffSize); err != nil {
			return nil, fmt.Errorf("error reading msg size for block request")
		}

		msgLen := binary.BigEndian.Uint32(pieceMsgBuffSize)

		pieceMsg := make([]byte, msgLen)
		if _, err := io.ReadFull(peerConn, pieceMsg); err != nil {
			return nil, fmt.Errorf("error reading block msg from peer")
		}

		block := pieceMsg[9:]
		blocks = append(blocks, block)
	}

	piece := slices.Concat(blocks...)

	return piece, nil
}
