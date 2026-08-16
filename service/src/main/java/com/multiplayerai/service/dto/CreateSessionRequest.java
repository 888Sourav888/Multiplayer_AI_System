package com.multiplayerai.service.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.UUID;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class CreateSessionRequest {

    @NotBlank(message = "Session name must not be blank")
    private String name;

    @NotNull(message = "Owner ID must not be null")
    private UUID ownerId;

    private String gitRepoUrl;
    private String gitBranch;
    private String gitCommitSha;

    public CreateSessionRequest(String name, UUID ownerId) {
        this.name = name;
        this.ownerId = ownerId;
        this.gitRepoUrl = null;
        this.gitBranch = null;
        this.gitCommitSha = null;
    }
}
