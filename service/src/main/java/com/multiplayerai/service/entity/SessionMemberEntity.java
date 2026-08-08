package com.multiplayerai.service.entity;

import com.multiplayerai.service.enums.MemberRole;
import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.OffsetDateTime;

@Entity
@Table(name = "session_members")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class SessionMemberEntity {

    @EmbeddedId
    private SessionMemberId id;

    @Enumerated(EnumType.STRING)
    @Column(name = "role", nullable = false)
    private MemberRole role = MemberRole.MEMBER;

    @Column(name = "joined_at", nullable = false, updatable = false)
    private OffsetDateTime joinedAt;

    @Column(name = "last_seen_at")
    private OffsetDateTime lastSeenAt;

    public SessionMemberEntity(SessionMemberId id, MemberRole role) {
        this.id = id;
        this.role = role;
    }

    @PrePersist
    protected void onCreate() {
        if (joinedAt == null) {
            joinedAt = OffsetDateTime.now();
        }
        if (role == null) {
            role = MemberRole.MEMBER;
        }
    }
}
