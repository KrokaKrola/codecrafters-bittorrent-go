package main

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
)

type bitTorrentFileInfo struct {
	length      int
	pieceLength int
	pieces      string
	piecesParts [][]byte
}

type bitTorrentFile struct {
	announce    string
	shaInfoHash string
	infoRaw     dictionary
	info        *bitTorrentFileInfo
	peers       []string
}

func parseBitTorrentFile(fileName string) (*bitTorrentFile, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to read file: %s", err.Error())
	}

	dec := decoder{
		data: data,
	}

	var dict dictionary

	if res, ok := dec.decode().(dictionary); ok {
		dict = res
	} else {
		return nil, fmt.Errorf("Invalid file content")
	}

	announce, ok := findElementInDictionary[string](dict, "announce")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, announce field was not found")
	}

	info, ok := findElementInDictionary[dictionary](dict, "info")
	if !ok {
		return nil, fmt.Errorf("Invalid file content, info field was not found")
	}

	encodedInfo, err := encode(info)
	if err != nil {
		fmt.Printf("Error while trying to encode info content: %s", err.Error())
		os.Exit(1)
	}

	hash, err := createSha1HashFromString(encodedInfo, false)
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

	return &bitTorrentFile{
		announce:    announce,
		infoRaw:     info,
		shaInfoHash: hash,
		info: &bitTorrentFileInfo{
			length:      infoLength,
			pieceLength: pieceLength,
			pieces:      pieces,
			piecesParts: piecesParts,
		},
	}, nil
}

func enrichWithPeers(btFile *bitTorrentFile) error {

	requestUrl, err := url.Parse(btFile.announce)
	if err != nil {
		return fmt.Errorf("Error while parsing announce URL: %s\n", err.Error())
	}

	requestQuery := requestUrl.Query()
	requestQuery.Set("info_hash", btFile.shaInfoHash)
	requestQuery.Set("peer_id", generateId())
	requestQuery.Set("port", "6881")
	requestQuery.Set("uploaded", "0")
	requestQuery.Set("downloaded", "0")
	requestQuery.Set("left", strconv.Itoa(btFile.info.length))
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

	bodyDecoder := &decoder{data: body}
	res := bodyDecoder.decode()

	dict, ok := res.(dictionary)
	if !ok {
		return fmt.Errorf("Invalid response from announce URL")
	}

	peers, ok := findElementInDictionary[string](dict, "peers")
	if !ok {
		return fmt.Errorf("Invalid response from announce URL, peers field is not in the response")
	}

	peersAsBytesArr := []byte(peers)
	var result []string

	for i := 0; i < len(peersAsBytesArr); i += 6 {
		ip1 := peersAsBytesArr[i]
		ip2 := peersAsBytesArr[i+1]
		ip3 := peersAsBytesArr[i+2]
		ip4 := peersAsBytesArr[i+3]
		port := binary.BigEndian.Uint16(peersAsBytesArr[i+4 : i+6])

		result = append(result, fmt.Sprintf("%d.%d.%d.%d:%d", ip1, ip2, ip3, ip4, port))
	}

	btFile.peers = result

	return nil
}

type peer struct {
	id   []byte
	conn net.Conn
}

func handshakeWithPeer(btFile *bitTorrentFile, address string) (*peer, error) {
	fmt.Println("handshake with", address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to establish TCP connection to: %s. Error: %s", address, err.Error())
	}

	message := []byte{19}
	reserved := make([]byte, 8)

	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, reserved...)
	message = append(message, []byte(btFile.shaInfoHash)...)
	message = append(message, []byte(generateId())...)

	_, err = conn.Write(message)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to send message to TCP connection: %s", err.Error())
	}

	res := make([]byte, 68)
	if _, err := io.ReadFull(conn, res); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error while trying to read message from TCP connection: %s", err.Error())
	}

	return &peer{
		id:   res[48:],
		conn: conn,
	}, nil
}

func unchokePeer(peer *peer) error {
	bitfieldMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(peer.conn, bitfieldMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading bitfield msg size from peer")
	}

	bitfieldMsg := make([]byte, binary.BigEndian.Uint32(bitfieldMsgSizeBuff))
	if _, err := io.ReadFull(peer.conn, bitfieldMsg); err != nil {
		return fmt.Errorf("error reading bitfield msg from peer")
	}

	intrestedMsg := []byte{}
	intrestedMsg = append(intrestedMsg, binary.BigEndian.AppendUint32(nil, 1)...)
	intrestedMsg = append(intrestedMsg, []byte{2}...)

	if _, err := peer.conn.Write(intrestedMsg); err != nil {
		return fmt.Errorf("error writing intrested msg to peer")
	}

	unchokeMsgSizeBuff := make([]byte, 4)
	if _, err := io.ReadFull(peer.conn, unchokeMsgSizeBuff); err != nil {
		return fmt.Errorf("error reading unchoke msg size from peer")
	}

	unchokeMsg := make([]byte, binary.BigEndian.Uint32(unchokeMsgSizeBuff))
	if _, err := io.ReadFull(peer.conn, unchokeMsg); err != nil {
		return fmt.Errorf("error reading unchoke msg from peer")
	}

	return nil
}

const infoBlockSize = 16384 // 16 kiB (16 * 1024 bytes)

func getPiece(btFile *bitTorrentFile, peer *peer, pieceIndex int) ([]byte, error) {
	blocks := [][]byte{}

	// last piece may be smaller than pieceLength if file size is not a multiple of pieceLength
	actualPieceLength := btFile.info.pieceLength
	if pieceIndex == len(btFile.info.piecesParts)-1 {
		actualPieceLength = btFile.info.length - (pieceIndex * btFile.info.pieceLength)
	}

	for begin := uint32(0); begin < uint32(actualPieceLength); begin += infoBlockSize {
		length := min(infoBlockSize, uint32(actualPieceLength)-begin)

		requestMsg := []byte{}

		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, 13)...)
		requestMsg = append(requestMsg, byte(6))
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, uint32(pieceIndex))...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, begin)...)
		requestMsg = append(requestMsg, binary.BigEndian.AppendUint32(nil, length)...)

		if _, err := peer.conn.Write(requestMsg); err != nil {
			return nil, fmt.Errorf("error writing request msg to peer")
		}

		pieceMsgBuffSize := make([]byte, 4)
		if _, err := io.ReadFull(peer.conn, pieceMsgBuffSize); err != nil {
			return nil, fmt.Errorf("error reading msg size for block request")
		}

		msgLen := binary.BigEndian.Uint32(pieceMsgBuffSize)

		pieceMsg := make([]byte, msgLen)
		if _, err := io.ReadFull(peer.conn, pieceMsg); err != nil {
			return nil, fmt.Errorf("error reading block msg from peer")
		}

		block := pieceMsg[9:]
		blocks = append(blocks, block)
	}

	piece := slices.Concat(blocks...)

	return piece, nil
}
