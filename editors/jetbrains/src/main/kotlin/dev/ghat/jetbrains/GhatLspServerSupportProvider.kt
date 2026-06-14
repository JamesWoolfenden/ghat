package dev.ghat.jetbrains

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.platform.lsp.api.LspServerSupportProvider
import com.intellij.platform.lsp.api.ProjectWideLspServerDescriptor

private val DOCKERFILE_RE = Regex("""(?i)^(dockerfile|containerfile)([._-].*)?$""")

private fun isGhatFile(file: VirtualFile): Boolean {
    val name = file.name
    if (name == ".gitlab-ci.yml" || name == ".gitlab-ci.yaml") return true
    if (name == ".pre-commit-config.yaml" || name == ".pre-commit-config.yml") return true
    if (name == "action.yml" || name == "action.yaml") return true
    if (file.extension == "dockerfile" || DOCKERFILE_RE.matches(name)) return true
    if ((file.extension == "yml" || file.extension == "yaml") &&
        file.parent?.name == "workflows" &&
        file.parent?.parent?.name == ".github"
    ) return true
    return false
}

class GhatLspServerSupportProvider : LspServerSupportProvider {
    override fun fileOpened(
        project: Project,
        file: VirtualFile,
        serverStarter: LspServerSupportProvider.LspServerStarter,
    ) {
        if (isGhatFile(file)) {
            serverStarter.ensureServerStarted(GhatLspServerDescriptor(project))
        }
    }
}

private class GhatLspServerDescriptor(project: Project) :
    ProjectWideLspServerDescriptor(project, "ghat") {

    override fun isSupportedFile(file: VirtualFile) = isGhatFile(file)

    override fun createCommandLine(): GeneralCommandLine {
        val binary = System.getenv("GHAT_BIN") ?: "ghat"
        return GeneralCommandLine(binary, "lsp")
    }
}
