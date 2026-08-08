package com.multiplayerai.service.dto;

import com.multiplayerai.service.entity.SnapshotEntity;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.OffsetDateTime;
import java.util.UUID;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class SnapshotResponse {

    private UUID id;
    private UUID sessionId;
    private Long version;
    private String storageLocation;
    private OffsetDateTime createdAt;

    public static SnapshotResponse fromEntity(SnapshotEntity entity) {
        SnapshotResponse response = new SnapshotResponse();
        response.setId(entity.getId());
        response.setSessionId(entity.getSessionId());
        response.setVersion(entity.getVersion());
        response.setStorageLocation(entity.getStorageLocation());
        response.setCreatedAt(entity.getCreatedAt());
        return response;
    }
}
