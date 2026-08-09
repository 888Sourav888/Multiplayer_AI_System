package com.multiplayerai.service.event;

import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.messaging.SessionConnectEvent;
import org.springframework.web.socket.messaging.SessionConnectedEvent;
import org.springframework.web.socket.messaging.SessionDisconnectEvent;
import org.springframework.web.socket.messaging.SessionSubscribeEvent;

@Component
public class WebSocketSessionEventListener {

    /**
     * Application Breakpoint for STOMP CONNECT frame reception.
     * Fires when Spring receives a STOMP CONNECT frame from a client.
     */
    @EventListener
    public void handleSessionConnect(SessionConnectEvent event) {
        StompHeaderAccessor headers = StompHeaderAccessor.wrap(event.getMessage());
        System.out.println("[WebSocket Event] Received STOMP CONNECT frame from sessionId: " + headers.getSessionId());
    }

    /**
     * Application Breakpoint for STOMP CONNECTED response completion.
     * Fires after Spring successfully validates connection and responds with CONNECTED frame.
     */
    @EventListener
    public void handleSessionConnected(SessionConnectedEvent event) {
        StompHeaderAccessor headers = StompHeaderAccessor.wrap(event.getMessage());
        System.out.println("[WebSocket Event] STOMP Client Connected successfully: " + headers.getSessionId());
    }

    /**
     * Application Breakpoint for STOMP SUBSCRIBE frame.
     * Fires when a client subscribes to a topic (e.g. /topic/session/{id}).
     */
    @EventListener
    public void handleSessionSubscribe(SessionSubscribeEvent event) {
        StompHeaderAccessor headers = StompHeaderAccessor.wrap(event.getMessage());
        System.out.println("[WebSocket Event] Client subscribed to destination: " + headers.getDestination());
    }

    /**
     * Application Breakpoint for STOMP DISCONNECT frame.
     */
    @EventListener
    public void handleSessionDisconnect(SessionDisconnectEvent event) {
        StompHeaderAccessor headers = StompHeaderAccessor.wrap(event.getMessage());
        System.out.println("[WebSocket Event] Client disconnected: " + headers.getSessionId());
    }
}
