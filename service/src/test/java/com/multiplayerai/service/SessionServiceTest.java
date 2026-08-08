package com.multiplayerai.service;

import com.multiplayerai.service.dto.CreateSessionRequest;
import com.multiplayerai.service.dto.SessionResponse;
import com.multiplayerai.service.dto.SnapshotResponse;
import com.multiplayerai.service.entity.SessionEntity;
import com.multiplayerai.service.entity.SnapshotEntity;
import com.multiplayerai.service.enums.SessionStatus;
import com.multiplayerai.service.exception.ResourceNotFoundException;
import com.multiplayerai.service.repository.SessionMemberRepository;
import com.multiplayerai.service.repository.SessionRepository;
import com.multiplayerai.service.repository.SnapshotRepository;
import com.multiplayerai.service.repository.UserRepository;
import com.multiplayerai.service.service.FileStorageService;
import com.multiplayerai.service.service.SessionService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.mock.web.MockMultipartFile;

import java.util.Optional;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class SessionServiceTest {

    @Mock
    private SessionRepository sessionRepository;
    @Mock
    private SessionMemberRepository sessionMemberRepository;
    @Mock
    private SnapshotRepository snapshotRepository;
    @Mock
    private UserRepository userRepository;
    @Mock
    private FileStorageService fileStorageService;

    @org.mockito.InjectMocks
    private SessionService sessionService;

    @Test
    void createSession_Success() {
        UUID ownerId = UUID.randomUUID();
        UUID sessionId = UUID.randomUUID();
        CreateSessionRequest request = new CreateSessionRequest("Test Session", ownerId);

        SessionEntity sessionEntity = new SessionEntity();
        sessionEntity.setId(sessionId);
        sessionEntity.setName("Test Session");
        sessionEntity.setOwnerId(ownerId);
        sessionEntity.setStatus(SessionStatus.ACTIVE);

        when(userRepository.findById(ownerId)).thenReturn(Optional.empty());
        when(sessionRepository.countByOwnerIdAndStatus(ownerId, SessionStatus.ACTIVE)).thenReturn(0L);
        when(sessionRepository.save(any(SessionEntity.class))).thenReturn(sessionEntity);
        when(fileStorageService.createSessionDirectory(any())).thenReturn("./data/sessions/" + sessionId);

        SessionResponse response = sessionService.createSession(request);

        assertNotNull(response);
        assertEquals("Test Session", response.getName());
        assertEquals(ownerId, response.getOwnerId());
        verify(sessionRepository, times(2)).save(any(SessionEntity.class));
        verify(sessionMemberRepository, times(1)).save(any());
    }

    @Test
    void persistSession_Success() {
        UUID sessionId = UUID.randomUUID();
        MockMultipartFile zipFile = new MockMultipartFile("file", "snapshot.zip", "application/zip", "dummy data".getBytes());

        SessionEntity sessionEntity = new SessionEntity();
        sessionEntity.setId(sessionId);
        sessionEntity.setCurrentVersion(1L);
        sessionEntity.setStatus(SessionStatus.ACTIVE);

        SnapshotEntity snapshotEntity = new SnapshotEntity(sessionId, 2L, "./data/sessions/" + sessionId + "/snapshots/snapshot_v2.zip");
        snapshotEntity.setId(UUID.randomUUID());

        when(sessionRepository.findById(sessionId)).thenReturn(Optional.of(sessionEntity));
        when(fileStorageService.storeSnapshot(eq(sessionId), eq(2L), any())).thenReturn("./data/sessions/" + sessionId + "/snapshots/snapshot_v2.zip");
        when(snapshotRepository.save(any(SnapshotEntity.class))).thenReturn(snapshotEntity);

        SnapshotResponse response = sessionService.persistSession(sessionId, zipFile);

        assertNotNull(response);
        assertEquals(2L, response.getVersion());
        assertEquals(2L, sessionEntity.getCurrentVersion());
        verify(sessionRepository, times(1)).save(sessionEntity);
    }

    @Test
    void deleteSession_Success() {
        UUID sessionId = UUID.randomUUID();
        SessionEntity sessionEntity = new SessionEntity();
        sessionEntity.setId(sessionId);
        sessionEntity.setStatus(SessionStatus.ACTIVE);

        when(sessionRepository.findById(sessionId)).thenReturn(Optional.of(sessionEntity));
        when(sessionRepository.save(any(SessionEntity.class))).thenReturn(sessionEntity);
        when(fileStorageService.deleteSessionDirectory(sessionId)).thenReturn(true);

        SessionResponse response = sessionService.deleteSession(sessionId);

        assertNotNull(response);
        assertEquals(SessionStatus.TERMINATED, response.getStatus());
        verify(fileStorageService, times(1)).deleteSessionDirectory(sessionId);
    }

    @Test
    void getSession_NotFound_ThrowsException() {
        UUID sessionId = UUID.randomUUID();
        when(sessionRepository.findById(sessionId)).thenReturn(Optional.empty());

        assertThrows(ResourceNotFoundException.class, () -> sessionService.getSession(sessionId));
    }
}
