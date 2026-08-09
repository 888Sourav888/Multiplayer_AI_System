package com.multiplayerai.service.event;

import com.multiplayerai.service.dto.PatchBroadcastResponse;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Component;

@Component
public class PatchBroadcasterObserver {

    @Autowired
    private SimpMessagingTemplate messagingTemplate;

    /**
     * Observer pattern event listener for incoming patch events.
     * Relays patches to all WebSocket clients subscribing to /topic/session/{sessionId}.
     */
    @EventListener
    public void handlePatchReceived(PatchReceivedEvent event) {
        String destination = "/topic/session/" + event.getSessionId();

        PatchBroadcastResponse broadcast = new PatchBroadcastResponse(
                "PATCH_BROADCAST",
                event.getSessionId(),
                event.getSenderId(),
                event.getPatchTimestamp(),
                event.getPatches()
        );

        messagingTemplate.convertAndSend(destination, broadcast);
    }
}
