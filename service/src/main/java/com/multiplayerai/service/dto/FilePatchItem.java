package com.multiplayerai.service.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@com.fasterxml.jackson.annotation.JsonIgnoreProperties(ignoreUnknown = true)
public class FilePatchItem {

    @NotBlank(message = "File path from root must not be blank")
    private String filePathFromRoot;

    private String fileName;
    
    private String fileExtension;
    
    private String operation;     // "CREATE", "WRITE", "REMOVE"
    
    private Long sizeBytes;
    
    private String modifier;      // e.g. "AI (via cursor)", "User"
    
    private boolean isAiEdit;
    
    private boolean isRevert;
    
    private boolean isWholeFile;
    
    private String contentDelta;  // JSON string representing added/removed lines
    
    private String fileChanges;   // Backwards-compatible raw change content
}
