package com.multiplayerai.service.entity;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.OffsetDateTime;
import java.util.UUID;

@Entity
@Table(name = "snapshots")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class SnapshotEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.AUTO)
    @Column(name = "id", nullable = false, updatable = false)
    private UUID id;

    @Column(name = "session_id", nullable = false)
    private UUID sessionId;

    @Column(name = "version", nullable = false)
    private Long version;

    @Column(name = "storage_location", nullable = false, columnDefinition = "TEXT")
    private String storageLocation;

    @Column(name = "created_at", nullable = false, updatable = false)
    private OffsetDateTime createdAt;

    public SnapshotEntity(UUID sessionId, Long version, String storageLocation) {
        this.sessionId = sessionId;
        this.version = version;
        this.storageLocation = storageLocation;
    }

    @PrePersist
    protected void onCreate() {
        if (createdAt == null) {
            createdAt = OffsetDateTime.now();
        }
    }
}
