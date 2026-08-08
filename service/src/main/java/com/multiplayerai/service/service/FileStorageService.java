package com.multiplayerai.service.service;

import com.multiplayerai.service.exception.StorageException;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.util.FileSystemUtils;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.util.UUID;

@Service
public class FileStorageService {

    private final Path rootStoragePath;

    public FileStorageService(@Value("${app.storage.base-dir:./data/sessions}") String baseDir) {
        this.rootStoragePath = Paths.get(baseDir).toAbsolutePath().normalize();
        try {
            Files.createDirectories(this.rootStoragePath);
        } catch (IOException e) {
            throw new StorageException("Could not initialize root storage directory: " + baseDir, e);
        }
    }

    /**
     * Initializes the project storage directory for a new session.
     */
    public String createSessionDirectory(UUID sessionId) {
        Path sessionPath = rootStoragePath.resolve(sessionId.toString());
        Path snapshotsPath = sessionPath.resolve("snapshots");
        try {
            Files.createDirectories(sessionPath);
            Files.createDirectories(snapshotsPath);
            return sessionPath.toString();
        } catch (IOException e) {
            throw new StorageException("Could not create session directory for session: " + sessionId, e);
        }
    }

    /**
     * Stores a snapshot zip/blob file for a session.
     */
    public String storeSnapshot(UUID sessionId, long version, MultipartFile file) {
        if (file == null || file.isEmpty()) {
            throw new StorageException("Failed to store empty snapshot file.");
        }

        Path sessionSnapshotsDir = rootStoragePath.resolve(sessionId.toString()).resolve("snapshots");
        try {
            if (!Files.exists(sessionSnapshotsDir)) {
                Files.createDirectories(sessionSnapshotsDir);
            }

            String originalFilename = file.getOriginalFilename();
            String extension = ".zip";
            if (originalFilename != null && originalFilename.contains(".")) {
                extension = originalFilename.substring(originalFilename.lastIndexOf("."));
            }

            String snapshotFileName = "snapshot_v" + version + extension;
            Path destinationPath = sessionSnapshotsDir.resolve(snapshotFileName);

            try (InputStream inputStream = file.getInputStream()) {
                Files.copy(inputStream, destinationPath, StandardCopyOption.REPLACE_EXISTING);
            }

            return destinationPath.toString();
        } catch (IOException e) {
            throw new StorageException("Failed to store snapshot file for session " + sessionId + " version " + version, e);
        }
    }

    /**
     * Deletes the session directory and all its contents.
     */
    public boolean deleteSessionDirectory(UUID sessionId) {
        Path sessionPath = rootStoragePath.resolve(sessionId.toString());
        try {
            if (Files.exists(sessionPath)) {
                return FileSystemUtils.deleteRecursively(sessionPath);
            }
            return true;
        } catch (IOException e) {
            throw new StorageException("Failed to delete storage directory for session: " + sessionId, e);
        }
    }

    public Path getRootStoragePath() {
        return rootStoragePath;
    }
}
