package com.multiplayerai.service.event;

import com.multiplayerai.service.dto.FilePatchItem;
import com.multiplayerai.service.dto.PatchBroadcastResponse;
import com.multiplayerai.service.entity.AiFileChangeEntity;
import com.multiplayerai.service.repository.AiFileChangeRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Component;

import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.ZoneId;

@Component
public class PatchBroadcasterObserver {

    @Autowired
    private SimpMessagingTemplate messagingTemplate;

    @Autowired
    private AiFileChangeRepository aiFileChangeRepository;

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

        // Persist patches flagged as AI edits to the database
        if (event.getPatches() != null) {
            for (FilePatchItem patch : event.getPatches()) {
                if (patch.isAiEdit()) {
                    try {
                        AiFileChangeEntity entity = new AiFileChangeEntity();
                        entity.setSessionId(event.getSessionId());
                        entity.setSenderId(event.getSenderId());
                        entity.setFilePath(patch.getFilePathFromRoot());
                        entity.setFileName(patch.getFileName());
                        entity.setFileExtension(patch.getFileExtension());
                        entity.setOperation(patch.getOperation());
                        entity.setSizeBytes(patch.getSizeBytes());
                        entity.setModifier(patch.getModifier() != null ? patch.getModifier() : "AI");
                        entity.setAiEdit(true);
                        entity.setRevert(patch.isRevert());
                        entity.setWholeFile(patch.isWholeFile());
                        entity.setContentDelta(patch.getContentDelta());
                        
                        // Map timestamp
                        Instant instant = Instant.ofEpochMilli(event.getPatchTimestamp());
                        entity.setEventTimestamp(OffsetDateTime.ofInstant(instant, ZoneId.systemDefault()));

                        aiFileChangeRepository.save(entity);
                    } catch (Exception e) {
                        e.printStackTrace();
                    }
                }
            }
        }
    }
}
