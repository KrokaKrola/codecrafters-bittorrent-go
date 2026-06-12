package torrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"

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
	infoRaw     Dictionary
	Info        *fileInfo
	Peers       []*Peer
}

func NewTorrentFile(fileName string) (*TorrentFile, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to read file: %s", err.Error())
	}

	dec := Decoder{
		Data: data,
	}

	var dict Dictionary

	if res, ok := dec.Decode().(Dictionary); ok {
		dict = res
	} else {
		return nil, fmt.Errorf("Invalid file content")
	}

	announce, ok := findElementInDictionary[string](dict, "announce")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, announce field was not found")
	}

	info, ok := findElementInDictionary[Dictionary](dict, "info")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, info field was not found")
	}

	encodedInfo, err := encode(info)
	if err != nil {
		fmt.Printf("Error while trying to encode info content: %s", err.Error())
		os.Exit(1)
	}

	hash, err := stringutil.CreateSha1HashFromString(encodedInfo, false)
	if err != nil {
		fmt.Println("Error while trying to create hash from info content")
		os.Exit(1)
	}

	infoLength, ok := findElementInDictionary[int](info, "length")
	if !ok {
		fmt.Println("Length field in info is not found")
		os.Exit(1)
	}

	pieceLength, ok := findElementInDictionary[int](info, "piece length")
	if !ok {
		fmt.Println("Piece length filed in info is not found")
		os.Exit(1)
	}

	pieces, ok := findElementInDictionary[string](info, "pieces")
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
	requestUrl, err := url.Parse(btFile.Announce)
	if err != nil {
		return fmt.Errorf("Error while parsing announce URL: %s\n", err.Error())
	}

	requestQuery := requestUrl.Query()
	requestQuery.Set("info_hash", btFile.ShaInfoHash)
	requestQuery.Set("peer_id", stringutil.GenerateId())
	requestQuery.Set("port", "6881")
	requestQuery.Set("uploaded", "0")
	requestQuery.Set("downloaded", "0")
	requestQuery.Set("left", strconv.Itoa(btFile.Info.Length))
	requestQuery.Set("compact", "1")

	requestUrl.RawQuery = requestQuery.Encode()

	client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("Error during request to announce URL: %s\n", err.Error())
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Error during request to announce URL: %s\n", err.Error())
	}

	body, err := io.ReadAll(response.Body)
	defer response.Body.Close()

	bodyDecoder := &Decoder{Data: body}
	res := bodyDecoder.Decode()

	dict, ok := res.(Dictionary)
	if !ok {
		return fmt.Errorf("Invalid response from announce URL")
	}

	peers, ok := findElementInDictionary[string](dict, "peers")
	if !ok {
		return fmt.Errorf("Invalid response from announce URL, peers field is not in the response")
	}

	peersAsBytesArr := []byte(peers)
	var result []*Peer

	for i := 0; i < len(peersAsBytesArr); i += 6 {
		ipPartOne := peersAsBytesArr[i]
		ipPartTwo := peersAsBytesArr[i+1]
		ipPartThree := peersAsBytesArr[i+2]
		ipPartFour := peersAsBytesArr[i+3]
		port := binary.BigEndian.Uint16(peersAsBytesArr[i+4 : i+6])

		result = append(result, &Peer{
			Addr: fmt.Sprintf("%d.%d.%d.%d:%d", ipPartOne, ipPartTwo, ipPartThree, ipPartFour, port),
		})
	}

	btFile.Peers = result

	return nil
}

func (btFile *TorrentFile) HandshakeWithPeer(address string) (*Peer, error) {
	fmt.Println("handshake with", address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to establish TCP connection to: %s. Error: %s", address, err.Error())
	}

	message := []byte{19}
	reserved := make([]byte, 8)

	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, reserved...)
	message = append(message, []byte(btFile.ShaInfoHash)...)
	message = append(message, []byte(stringutil.GenerateId())...)

	if _, err = conn.Write(message); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to send message to TCP connection: %s", err.Error())
	}

	res := make([]byte, 68)
	if _, err := io.ReadFull(conn, res); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to read message from TCP connection: %s", err.Error())
	}

	return &Peer{Addr: address, Conn: conn, Id: res[48:]}, nil
}

const infoBlockSize = 16384 // 16 kiB (16 * 1024 bytes)

func (btFile *TorrentFile) GetPiece(peer *Peer, pieceIndex int) ([]byte, error) {
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

		if _, err := peer.Conn.Write(requestMsg); err != nil {
			return nil, fmt.Errorf("error writing request msg to peer")
		}

		pieceMsgBuffSize := make([]byte, 4)
		if _, err := io.ReadFull(peer.Conn, pieceMsgBuffSize); err != nil {
			return nil, fmt.Errorf("error reading msg size for block request")
		}

		msgLen := binary.BigEndian.Uint32(pieceMsgBuffSize)

		pieceMsg := make([]byte, msgLen)
		if _, err := io.ReadFull(peer.Conn, pieceMsg); err != nil {
			return nil, fmt.Errorf("error reading block msg from peer")
		}

		block := pieceMsg[9:]
		blocks = append(blocks, block)
	}

	piece := slices.Concat(blocks...)

	return piece, nil
}
