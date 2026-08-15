package com.multiplayerai.service.handler;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.multiplayerai.service.dto.FilePatchItem;
import com.multiplayerai.service.dto.PatchBroadcastResponse;
import com.multiplayerai.service.dto.PatchTransferRequest;
import com.multiplayerai.service.event.PatchReceivedEvent;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.context.event.EventListener;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.CloseStatus;
import org.springframework.web.socket.TextMessage;
import org.springframework.web.socket.WebSocketSession;
import org.springframework.web.socket.handler.TextWebSocketHandler;

import java.io.IOException;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;

@Component
public class PlainJsonWebSocketHandler extends TextWebSocketHandler {

    @Autowired
    private ApplicationEventPublisher eventPublisher;

    private final ObjectMapper objectMapper = new ObjectMapper();

    // Map of sessionId -> Set of active WebSocketSessions
    private final Map<UUID, Set<WebSocketSession>> roomSessions = new ConcurrentHashMap<>();
    // Map of WebSocketSession.getId() -> subscribed sessionId
    private final Map<String, UUID> sessionToRoomMap = new ConcurrentHashMap<>();

    private void sendJsonMessage(WebSocketSession session, Map<String, Object> payload) throws IOException {
        session.sendMessage(new TextMessage(objectMapper.writeValueAsString(payload)));
    }

    @Override
    public void afterConnectionEstablished(WebSocketSession session) throws Exception {
        System.out.println("[WebSocket Event] Insomnia Client Connected: " + session.getId());
        Map<String, Object> connAck = new HashMap<>();
        connAck.put("status", "CONNECTED");
        connAck.put("connectionId", session.getId());
        connAck.put("message", "Connected to Multiplayer WebSocket Server");
        sendJsonMessage(session, connAck);
    }

    @Override
    protected void handleTextMessage(WebSocketSession session, TextMessage message) throws Exception {
        String payload = message.getPayload();
        System.out.println("[WebSocket Event] Received message from Insomnia: " + payload);

        try {
            JsonNode jsonNode = objectMapper.readTree(payload);
            String type = jsonNode.has("type") ? jsonNode.get("type").asText() : "PATCH_TRANSFER";

            if ("SUBSCRIBE".equalsIgnoreCase(type) || "JOIN".equalsIgnoreCase(type)) {
                UUID sessionId = UUID.fromString(jsonNode.get("sessionId").asText());
                subscribeSessionToRoom(session, sessionId);
                Map<String, Object> subAck = new HashMap<>();
                subAck.put("status", "SUBSCRIBED");
                subAck.put("sessionId", sessionId);
                sendJsonMessage(session, subAck);
                return;
            }

            // Default or PATCH_TRANSFER handling
            PatchTransferRequest request = objectMapper.readValue(payload, PatchTransferRequest.class);

            if (request.getSessionId() != null) {
                subscribeSessionToRoom(session, request.getSessionId());
            }

            // Publish event to trigger Observer pattern subscribers
            PatchReceivedEvent event = new PatchReceivedEvent(
                    this,
                    request.getSessionId(),
                    request.getSenderId(),
                    request.getTimestamp(),
                    request.getPatches()
            );
            eventPublisher.publishEvent(event);

            // Acknowledge sender
            Map<String, Object> ack = new HashMap<>();
            ack.put("status", "PATCH_RECEIVED");
            ack.put("message", "Patch broadcasted to session participants");
            sendJsonMessage(session, ack);

        } catch (Exception e) {
            Map<String, Object> err = new HashMap<>();
            err.put("status", "ERROR");
            err.put("error", e.getMessage());
            sendJsonMessage(session, err);
        }
    }

    private void subscribeSessionToRoom(WebSocketSession session, UUID sessionId) {
        roomSessions.computeIfAbsent(sessionId, k -> ConcurrentHashMap.newKeySet()).add(session);
        sessionToRoomMap.put(session.getId(), sessionId);
        System.out.println("[WebSocket Event] Client " + session.getId() + " subscribed to session: " + sessionId);
    }

    @EventListener
    public void onPatchReceived(PatchReceivedEvent event) {
        UUID sessionId = event.getSessionId();
        Set<WebSocketSession> sessions = roomSessions.get(sessionId);
        if (sessions == null || sessions.isEmpty()) {
            return;
        }

        PatchBroadcastResponse broadcast = new PatchBroadcastResponse(
                "PATCH_BROADCAST",
                event.getSessionId(),
                event.getSenderId(),
                event.getPatchTimestamp(),
                event.getPatches()
        );

        try {
            String broadcastJson = objectMapper.writeValueAsString(broadcast);
            TextMessage message = new TextMessage(broadcastJson);
            for (WebSocketSession session : sessions) {
                if (session.isOpen()) {
                    session.sendMessage(message);
                }
            }
        } catch (IOException e) {
            e.printStackTrace();
        }
    }

    @Override
    public void afterConnectionClosed(WebSocketSession session, CloseStatus status) throws Exception {
        UUID sessionId = sessionToRoomMap.remove(session.getId());
        if (sessionId != null) {
            Set<WebSocketSession> sessions = roomSessions.get(sessionId);
            if (sessions != null) {
                sessions.remove(session);
            }
        }
        System.out.println("[WebSocket Event] Insomnia Client Disconnected: " + session.getId());
    }
}
