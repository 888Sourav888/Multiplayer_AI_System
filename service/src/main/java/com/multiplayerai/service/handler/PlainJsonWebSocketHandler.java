package com.multiplayerai.service.handler;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.multiplayerai.service.dto.FilePatchItem;
import com.multiplayerai.service.dto.PatchBroadcastResponse;
import com.multiplayerai.service.dto.PatchTransferRequest;
import com.multiplayerai.service.event.PatchReceivedEvent;
import com.multiplayerai.service.entity.SessionEntity;
import com.multiplayerai.service.entity.SessionMemberId;
import com.multiplayerai.service.entity.SessionMemberEntity;
import com.multiplayerai.service.repository.SessionRepository;
import com.multiplayerai.service.repository.SessionMemberRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.context.event.EventListener;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.CloseStatus;
import org.springframework.web.socket.TextMessage;
import org.springframework.web.socket.WebSocketSession;
import org.springframework.web.socket.handler.TextWebSocketHandler;

import java.io.IOException;
import java.time.OffsetDateTime;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;

@Component
public class PlainJsonWebSocketHandler extends TextWebSocketHandler {

    @Autowired
    private ApplicationEventPublisher eventPublisher;

    @Autowired
    private SessionMemberRepository sessionMemberRepository;

    @Autowired
    private SessionRepository sessionRepository;

    private final ObjectMapper objectMapper = new ObjectMapper();

    // Map of sessionId -> Set of active WebSocketSessions
    private final Map<UUID, Set<WebSocketSession>> roomSessions = new ConcurrentHashMap<>();
    // Map of WebSocketSession.getId() -> subscribed sessionId
    private final Map<String, UUID> sessionToRoomMap = new ConcurrentHashMap<>();

    public static class UserRoomKey {
        public final UUID sessionId;
        public final UUID userId;
        public UserRoomKey(UUID sessionId, UUID userId) {
            this.sessionId = sessionId;
            this.userId = userId;
        }
    }

    private final Map<String, UserRoomKey> sessionToUserRoomMap = new ConcurrentHashMap<>();

    private void sendJsonMessage(WebSocketSession session, Map<String, Object> payload) throws IOException {
        session.sendMessage(new TextMessage(objectMapper.writeValueAsString(payload)));
    }

    @Override
    public void afterConnectionEstablished(WebSocketSession session) throws Exception {
        System.out.println("[WebSocket Event] Client Connected: " + session.getId());
        Map<String, Object> connAck = new HashMap<>();
        connAck.put("status", "CONNECTED");
        connAck.put("connectionId", session.getId());
        connAck.put("message", "Connected to Multiplayer WebSocket Server");
        sendJsonMessage(session, connAck);
    }

    @Override
    protected void handleTextMessage(WebSocketSession session, TextMessage message) throws Exception {
        String payload = message.getPayload();
        System.out.println("[WebSocket Event] Received message: " + payload);

        try {
            JsonNode jsonNode = objectMapper.readTree(payload);
            String type = jsonNode.has("type") ? jsonNode.get("type").asText() : "PATCH_TRANSFER";

            if ("SUBSCRIBE".equalsIgnoreCase(type) || "JOIN".equalsIgnoreCase(type)) {
                UUID sessionId = UUID.fromString(jsonNode.get("sessionId").asText());
                UUID userId = jsonNode.has("senderId") ? UUID.fromString(jsonNode.get("senderId").asText()) : null;

                // Enforce owner connection presence check for non-owner WebSocket subscribers
                if (userId != null) {
                    SessionEntity sessionEntity = sessionRepository.findById(sessionId).orElse(null);
                    if (sessionEntity != null) {
                        UUID ownerId = sessionEntity.getOwnerId();
                        if (!userId.equals(ownerId)) {
                            SessionMemberId ownerMemberId = new SessionMemberId(sessionId, ownerId);
                            SessionMemberEntity ownerMember = sessionMemberRepository.findById(ownerMemberId).orElse(null);
                            if (ownerMember == null || !Boolean.TRUE.equals(ownerMember.getIsConnected())) {
                                Map<String, Object> err = new HashMap<>();
                                err.put("status", "ERROR");
                                err.put("error", "Cannot subscribe to session. The session owner is not currently connected.");
                                sendJsonMessage(session, err);
                                return;
                            }
                        }
                    }
                }

                subscribeSessionToRoom(session, sessionId, userId);
                Map<String, Object> subAck = new HashMap<>();
                subAck.put("status", "SUBSCRIBED");
                subAck.put("sessionId", sessionId);
                sendJsonMessage(session, subAck);
                return;
            }

            // Default or PATCH_TRANSFER handling
            PatchTransferRequest request = objectMapper.readValue(payload, PatchTransferRequest.class);

            if (request.getSessionId() != null) {
                // Enforce owner connection presence check for non-owner implicit subscribers
                UUID sessionId = request.getSessionId();
                UUID userId = request.getSenderId();
                if (userId != null) {
                    SessionEntity sessionEntity = sessionRepository.findById(sessionId).orElse(null);
                    if (sessionEntity != null) {
                        UUID ownerId = sessionEntity.getOwnerId();
                        if (!userId.equals(ownerId)) {
                            SessionMemberId ownerMemberId = new SessionMemberId(sessionId, ownerId);
                            SessionMemberEntity ownerMember = sessionMemberRepository.findById(ownerMemberId).orElse(null);
                            if (ownerMember == null || !Boolean.TRUE.equals(ownerMember.getIsConnected())) {
                                Map<String, Object> err = new HashMap<>();
                                err.put("status", "ERROR");
                                err.put("error", "Cannot subscribe to session. The session owner is not currently connected.");
                                sendJsonMessage(session, err);
                                return;
                            }
                        }
                    }
                }
                subscribeSessionToRoom(session, request.getSessionId(), request.getSenderId());
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

    private void subscribeSessionToRoom(WebSocketSession session, UUID sessionId, UUID userId) {
        roomSessions.computeIfAbsent(sessionId, k -> ConcurrentHashMap.newKeySet()).add(session);
        sessionToRoomMap.put(session.getId(), sessionId);
        if (userId != null) {
            sessionToUserRoomMap.put(session.getId(), new UserRoomKey(sessionId, userId));
            // Update connection status in the database (multi-server safe)
            try {
                SessionMemberId memberId = new SessionMemberId(sessionId, userId);
                sessionMemberRepository.findById(memberId).ifPresent(member -> {
                    member.setIsConnected(true);
                    member.setLastSeenAt(OffsetDateTime.now());
                    sessionMemberRepository.save(member);
                    System.out.println("[WebSocket Event] Database updated: User " + userId + " is CONNECTED to session " + sessionId);
                });
            } catch (Exception e) {
                System.err.println("[WebSocket Event] Failed to update user connection in DB: " + e.getMessage());
            }
        } else {
            System.out.println("[WebSocket Event] Client " + session.getId() + " subscribed to session: " + sessionId);
        }
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
        UserRoomKey key = sessionToUserRoomMap.remove(session.getId());
        if (key != null) {
            // Check if there is any other active connection in the map for this user and session
            boolean stillConnected = false;
            for (UserRoomKey val : sessionToUserRoomMap.values()) {
                if (val.sessionId.equals(key.sessionId) && val.userId.equals(key.userId)) {
                    stillConnected = true;
                    break;
                }
            }

            if (!stillConnected) {
                // Update connection status in the database to false (multi-server safe)
                try {
                    SessionMemberId memberId = new SessionMemberId(key.sessionId, key.userId);
                    sessionMemberRepository.findById(memberId).ifPresent(member -> {
                        member.setIsConnected(false);
                        member.setLastSeenAt(OffsetDateTime.now());
                        sessionMemberRepository.save(member);
                        System.out.println("[WebSocket Event] Database updated: User " + key.userId + " is DISCONNECTED from session " + key.sessionId);
                    });
                } catch (Exception e) {
                    System.err.println("[WebSocket Event] Failed to update user disconnection in DB: " + e.getMessage());
                }
            } else {
                System.out.println("[WebSocket Event] Connection closed, but User " + key.userId + " still has other active connections in session " + key.sessionId);
            }
        } else {
            System.out.println("[WebSocket Event] Client Disconnected: " + session.getId());
        }
    }
}
