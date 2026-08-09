package com.multiplayerai.service.controller;

import com.multiplayerai.service.dto.PatchBroadcastResponse;
import com.multiplayerai.service.dto.PatchTransferRequest;
import com.multiplayerai.service.event.PatchReceivedEvent;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.UUID;

@RestController
@RequestMapping("/api/sessions")
public class SessionPatchRestController {

    @Autowired
    private ApplicationEventPublisher eventPublisher;

    /**
     * REST endpoint to submit patches via HTTP POST.
     * Triggers PatchReceivedEvent, notifying Observer listeners (broadcasting to WebSocket clients).
     */
    @PostMapping("/{sessionId}/patches")
    public ResponseEntity<PatchBroadcastResponse> submitPatch(
            @PathVariable UUID sessionId,
            @Valid @RequestBody PatchTransferRequest request) {

        request.setSessionId(sessionId);

        PatchReceivedEvent event = new PatchReceivedEvent(
                this,
                request.getSessionId(),
                request.getSenderId(),
                request.getTimestamp(),
                request.getPatches()
        );

        // Publish event to trigger Observer pattern subscribers
        eventPublisher.publishEvent(event);

        PatchBroadcastResponse response = new PatchBroadcastResponse(
                "PATCH_BROADCAST",
                request.getSessionId(),
                request.getSenderId(),
                request.getTimestamp(),
                request.getPatches()
        );

        return ResponseEntity.ok(response);
    }
}
