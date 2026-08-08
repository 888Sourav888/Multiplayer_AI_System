package com.multiplayerai.service.dto;

import com.multiplayerai.service.entity.SessionEntity;
import com.multiplayerai.service.enums.SessionStatus;
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
public class SessionResponse {

    private UUID id;
    private String name;
    private UUID ownerId;
    private String projectStoragePath;
    private Long currentVersion;
    private SessionStatus status;
    private OffsetDateTime createdAt;
    private OffsetDateTime lastActiveAt;

    public static SessionResponse fromEntity(SessionEntity entity) {
        SessionResponse response = new SessionResponse();
        response.setId(entity.getId());
        response.setName(entity.getName());
        response.setOwnerId(entity.getOwnerId());
        response.setProjectStoragePath(entity.getProjectStoragePath());
        response.setCurrentVersion(entity.getCurrentVersion());
        response.setStatus(entity.getStatus());
        response.setCreatedAt(entity.getCreatedAt());
        response.setLastActiveAt(entity.getLastActiveAt());
        return response;
    }
}
