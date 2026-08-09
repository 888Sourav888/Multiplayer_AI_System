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
public class FilePatchItem {

    @NotBlank(message = "File path from root must not be blank")
    private String filePathFromRoot;

    @NotBlank(message = "File changes content must not be blank")
    private String fileChanges;
}
