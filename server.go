package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// SWOP TO LOCAL TMP DIR WHEN RUNNING LOCALLY
const folderPath = "/tmp/data/codecrafters.io/http-server-tester/"
const port = "4221"
const acceptEncoding = "Accept-Encoding"

var acceptedEndPoints = []string{"/echo", "/user-agent", "/files"}

var allowedEncoders = []string{"gzip"}
var allowedEncoderMap = map[string]bool{
	"gzip": true,
}

type AcceptedEndpoint struct {
	accepted     bool
	endPointBase string
}

func main() {
	fmt.Println("Logs from your program will appear here!")

	listener, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		fmt.Println("Failed to bind to port " + port)
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		return
	}

	headers, err := parseHeaders(reader)
	if err != nil {
		fmt.Println("Error reading headers: ", err.Error())
		return
	}

	response := processRequest(requestLine, headers, reader)
	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing response: ", err.Error())
	}
}

func parseHeaders(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}

	return headers, nil
}

func processRequest(requestLine string, headers map[string]string, reader *bufio.Reader) string {
	splitLine := strings.Split(requestLine, " ")
	if len(splitLine) < 2 {
		return "HTTP/1.1 400 Bad Request\r\n\r\n"
	}
	targetPath := splitLine[1]
	if targetPath == "/" {
		return "HTTP/1.1 200 OK\r\n\r\n"
	}

	acceptedEndpoint := isAcceptedEndPoint(targetPath)
	if !acceptedEndpoint.accepted {
		return "HTTP/1.1 404 Not Found\r\n\r\n"
	}

	httpMethod := splitLine[0]
	if httpMethod == "GET" {
		return processGet(acceptedEndpoint, headers, targetPath)
	} else if httpMethod == "POST" {
		body, _ := getBody(headers, reader)
		return processPost(acceptedEndpoint, headers, targetPath, body)
	}

	return "HTTP/1.1 400 Bad Request\r\n\r\n"
}

func processPost(acceptedEndpoint AcceptedEndpoint, headers map[string]string, targetPath string, body string) string {
	switch acceptedEndpoint.endPointBase {
	case "/files":
		withoutPath := removePath(targetPath, acceptedEndpoint)
		written := writeFile(withoutPath, body)
		if written {
			return "HTTP/1.1 201 Created\r\n\r\n"
		}
	}

	return ""
}

func processGet(acceptedEndpoint AcceptedEndpoint, headers map[string]string, targetPath string) string {
	switch acceptedEndpoint.endPointBase {
	case "/files":
		content, err := readFileContent(targetPath)
		fmt.Println(content)
		if err != nil {
			return "HTTP/1.1 404 Not Found\r\n\r\n"
		}
		return formatResponse(content, "application/octet-stream", "200", headers)
	case "/user-agent":
		userAgent := headers["User-Agent"]
		return formatResponse(userAgent, "text/plain", "200", headers)
	default:
		withoutPath := removePath(targetPath, acceptedEndpoint)
		return formatResponse(withoutPath, "text/plain", "200", headers)
	}
}

func formatResponse(content string, contentType string, httpStatus string, headers map[string]string) string {
	contentLength := strconv.Itoa(len(content))
	encodingFound, ok := headers[acceptEncoding]
	if !ok {
		parsedStr := fmt.Sprintf("HTTP/1.1 %s OK\r\nContent-Type: %s\r\nContent-Length: %s \r\n\r\n%s", httpStatus, contentType, contentLength, content)
		return strings.TrimSpace(parsedStr)
	}
	parsedEncoding := getValidEncoders(strings.Split(encodingFound, ","))
	fmt.Println(parsedEncoding)
	if len(parsedEncoding) >= 1 {
		zippedContent := compressString(content)
		fmt.Println(zippedContent)
		parsedStr := fmt.Sprintf("HTTP/1.1 %s OK\r\nContent-Type: %s\r\nContent-Encoding: %s \r\nContent-Length: %s\r\n\r\n%s", httpStatus, contentType, parsedEncoding, strconv.Itoa(len(zippedContent)), zippedContent)
		return strings.TrimSpace(parsedStr)
	} else {
		parsedStr := fmt.Sprintf("HTTP/1.1 %s OK\r\nContent-Type: text/plain\r\n\r\n%s", httpStatus, content)
		return strings.TrimSpace(parsedStr)
	}
}

func isAcceptedEndPoint(targetPath string) AcceptedEndpoint {
	for _, endpoint := range acceptedEndPoints {
		if strings.HasPrefix(targetPath, endpoint) {
			return AcceptedEndpoint{true, endpoint}
		}
	}
	return AcceptedEndpoint{false, ""}
}

func readFileContent(queryFilePath string) (string, error) {
	fileName := strings.TrimPrefix(queryFilePath, "/files/")
	filePath := fmt.Sprintf("%s/%s", folderPath, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error found")
		return "", err
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return content.String(), nil
}

func writeFile(queryFilePath string, content string) bool {
	fileName := strings.TrimPrefix(queryFilePath, "/files/")
	filePath := fmt.Sprintf("%s/%s", folderPath, fileName)
	file, errs := os.Create(filePath)
	if errs != nil {
		fmt.Println("Failed to create file:", errs)
		return false
	}
	defer file.Close()

	// Write the string "Hello, World!" to the file
	_, errs = file.WriteString(content)
	if errs != nil {
		fmt.Println("Failed to write to file:", errs) //print the failed message
		return false
	}
	return true
}

func getBody(headers map[string]string, reader *bufio.Reader) (string, error) {
	var body string
	if contentLength, ok := headers["Content-Length"]; ok {
		bodyLength := 0
		fmt.Sscanf(contentLength, "%d", &bodyLength)
		bodyBytes := make([]byte, bodyLength)
		_, err := reader.Read(bodyBytes)
		if err != nil {
			fmt.Println("Error reading body: ", err.Error())
			return "", err
		}
		body = string(bodyBytes)
	}

	return body, nil
}

func removePath(targetPath string, acceptedEndpoint AcceptedEndpoint) string {
	return strings.TrimSpace(strings.Replace(targetPath, acceptedEndpoint.endPointBase+"/", "", 1))
}

func getValidEncoders(arr []string) string {
	var validArr []string
	for _, value := range arr {
		parsed := strings.TrimSpace(value)
		found, ok := allowedEncoderMap[parsed]
		if ok && found {
			validArr = append(validArr, parsed)
		}
	}

	return strings.Join(validArr, "")
}

// compressString compresses a string using gzip and returns the compressed bytes
func compressString(content string) string {
	var buffer bytes.Buffer
	w := gzip.NewWriter(&buffer)
	w.Write([]byte(content))
	w.Close()
	content = buffer.String()
	return content
}

// toHex converts bytes to a hexadecimal string
func toHex(data []byte) string {
	return hex.EncodeToString(data)
}
