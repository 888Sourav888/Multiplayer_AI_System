package com.multiplayerai.service.event;

import com.multiplayerai.service.dto.FilePatchItem;
import lombok.Getter;
import org.springframework.context.ApplicationEvent;

import java.util.List;
import java.util.UUID;

@Getter
public class PatchReceivedEvent extends ApplicationEvent {

    private final UUID sessionId;
    private final UUID senderId;
    private final long patchTimestamp;
    private final List<FilePatchItem> patches;

    public PatchReceivedEvent(Object source, UUID sessionId, UUID senderId, long patchTimestamp, List<FilePatchItem> patches) {
        super(source);
        this.sessionId = sessionId;
        this.senderId = senderId;
        this.patchTimestamp = patchTimestamp;
        this.patches = patches;
    }
}
