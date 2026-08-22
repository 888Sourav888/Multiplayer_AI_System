package com.multiplayerai.service.controller;

import com.multiplayerai.service.dto.PatchTransferRequest;
import com.multiplayerai.service.event.PatchReceivedEvent;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.messaging.handler.annotation.DestinationVariable;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.stereotype.Controller;

import java.util.UUID;

@Controller
public class SessionPatchWebSocketController {

    @Autowired
    private ApplicationEventPublisher eventPublisher;

    /**
     * WebSocket Message Handler: /app/session/{sessionId}/patch
     * Receives incoming patch payload from a client, publishes PatchReceivedEvent,
     * triggering Observer listeners to broadcast and process the patch.
     */
    @MessageMapping("/session/{sessionId}/patch")
    public void receivePatch(
            @DestinationVariable("sessionId") UUID sessionId,
            @Valid @Payload PatchTransferRequest request) {

        // Ensure sessionId matches path variable
        request.setSessionId(sessionId);

        // Publish event to trigger Observer pattern subscribers
        PatchReceivedEvent event = new PatchReceivedEvent(
                this,
                request.getSessionId(),
                request.getSenderId(),
                request.getTimestamp(),
                request.getPatches()
        );

        eventPublisher.publishEvent(event);
    }
}
