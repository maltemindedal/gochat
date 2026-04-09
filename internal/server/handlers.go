// Package server exposes HTTP handlers, including WebSocket upgrades, health
// checks, and the built-in test page.
package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

func webSocketHandlerForHub(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed. WebSocket endpoint only accepts GET requests.", http.StatusMethodNotAllowed)
			return
		}

		if err := r.Context().Err(); err != nil {
			log.Print("WebSocket request cancelled before upgrade")
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}

		client := NewClient(conn, h, r.RemoteAddr)

		select {
		case client.hub.register <- client:
		case <-client.hub.shutdown:
			log.Print("Rejected WebSocket client because the hub is shutting down")
			if closeErr := conn.Close(); closeErr != nil && !isExpectedCloseError(closeErr) {
				log.Printf("Error closing rejected WebSocket connection: %v", closeErr)
			}
		case <-r.Context().Done():
			if closeErr := conn.Close(); closeErr != nil && !isExpectedCloseError(closeErr) {
				log.Printf("Error closing cancelled WebSocket connection: %v", closeErr)
			}
		}
	}
}

// WebSocketHandler handles WebSocket upgrade requests and manages client connections.
// It validates that the request uses the GET method, upgrades the HTTP connection
// to WebSocket, creates a new Client instance, and starts the client's read/write pumps.
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	webSocketHandlerForHub(GetHub()).ServeHTTP(w, r)
}

// HealthHandler provides a simple health check endpoint that returns server status.
// It responds with a plain text message indicating the server is running.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.Context().Err(); err != nil {
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprint(w, "GoChat server is running!"); err != nil {
		log.Printf("Error writing health response: %v", err)
	}
}

// TestPageHandler serves an HTML test page for testing WebSocket functionality.
// It provides a simple web interface to connect to the WebSocket endpoint,
// send messages, and view real-time chat communication.
func TestPageHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.Context().Err(); err != nil {
		return
	}

	w.Header().Set("Content-Type", "text/html")
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GoChat WebSocket Test</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        #messages { 
            border: 1px solid #ccc; 
            height: 300px; 
            padding: 10px; 
            overflow-y: scroll; 
            margin: 10px 0;
            background-color: #f9f9f9;
        }
        input[type="text"] { 
            width: 300px; 
            padding: 5px; 
            margin-right: 10px;
        }
        button { 
            padding: 5px 15px; 
            background-color: #007cba; 
            color: white; 
            border: none; 
            cursor: pointer;
        }
        button:hover { background-color: #005a87; }
        .status { 
            margin: 10px 0; 
            padding: 5px; 
            border-radius: 3px;
        }
        .connected { background-color: #d4edda; color: #155724; }
        .disconnected { background-color: #f8d7da; color: #721c24; }
    </style>
</head>
<body>
    <h1>GoChat WebSocket Test</h1>
    
    <div id="status" class="status disconnected">Disconnected</div>
    
    <div>
        <input type="text" id="messageInput" placeholder="Type a message..." disabled>
        <button id="sendButton" onclick="sendMessage()" disabled>Send</button>
        <button id="connectButton" onclick="toggleConnection()">Connect</button>
    </div>
    
    <div id="messages"></div>

    <script>
        let ws = null;
        const messagesDiv = document.getElementById('messages');
        const messageInput = document.getElementById('messageInput');
        const sendButton = document.getElementById('sendButton');
        const connectButton = document.getElementById('connectButton');
        const statusDiv = document.getElementById('status');

        function addMessage(message, type = 'info') {
            const messageElement = document.createElement('div');
            messageElement.style.margin = '5px 0';
            messageElement.style.padding = '3px';
            
            if (type === 'sent') {
                messageElement.style.color = 'blue';
                messageElement.innerHTML = '<strong>You:</strong> ' + message;
            } else if (type === 'received') {
                messageElement.style.color = 'green';
                messageElement.innerHTML = '<strong>Other:</strong> ' + message;
            } else {
                messageElement.style.color = 'gray';
                messageElement.innerHTML = '<em>' + message + '</em>';
            }
            
            messagesDiv.appendChild(messageElement);
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
        }

        function updateStatus(connected) {
            if (connected) {
                statusDiv.textContent = 'Connected';
                statusDiv.className = 'status connected';
                messageInput.disabled = false;
                sendButton.disabled = false;
                connectButton.textContent = 'Disconnect';
            } else {
                statusDiv.textContent = 'Disconnected';
                statusDiv.className = 'status disconnected';
                messageInput.disabled = true;
                sendButton.disabled = true;
                connectButton.textContent = 'Connect';
            }
        }

        function connect() {
            ws = new WebSocket('ws://localhost:8080/ws');
            
            ws.onopen = function(event) {
                addMessage('Connected to GoChat server');
                updateStatus(true);
            };
            
            ws.onmessage = function(event) {
                addMessage(event.data, 'received');
            };
            
            ws.onclose = function(event) {
                addMessage('Connection closed');
                updateStatus(false);
                ws = null;
            };
            
            ws.onerror = function(error) {
                addMessage('Connection error: ' + error);
                updateStatus(false);
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
            }
        }

        function toggleConnection() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                disconnect();
            } else {
                connect();
            }
        }

        function sendMessage() {
            const message = messageInput.value.trim();
            if (message && ws && ws.readyState === WebSocket.OPEN) {
                ws.send(message);
                addMessage(message, 'sent');
                messageInput.value = '';
            }
        }

        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    </script>
</body>
</html>`
	if _, err := fmt.Fprint(w, html); err != nil {
		log.Printf("Error writing HTML response: %v", err)
	}
}
