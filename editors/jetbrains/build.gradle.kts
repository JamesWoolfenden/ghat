plugins {
    id("org.jetbrains.kotlin.jvm") version "2.2.0"
    id("org.jetbrains.intellij.platform") version "2.9.0"
}

version = providers.gradleProperty("pluginVersion").getOrElse("0.1.0")

repositories {
    mavenCentral()
    intellijPlatform { defaultRepositories() }
}

dependencies {
    intellijPlatform {
        // LSP API lives in the Ultimate module set; building against IU keeps the
        // plugin loadable in GoLand / PyCharm Pro / WebStorm etc. as well.
        intellijIdeaUltimate("2024.3")
    }
}

kotlin {
    jvmToolchain(21)
}

intellijPlatform {
    instrumentCode = false
    buildSearchableOptions = false
    pluginConfiguration {
        id = "dev.ghat.jetbrains"
        name = "ghat"
        version = providers.gradleProperty("pluginVersion").getOrElse("0.1.0")
        description = "Pin GitHub Actions, GitLab CI images, pre-commit revs and Dockerfile FROMs to immutable SHAs via the ghat LSP server."
        ideaVersion {
            sinceBuild = "243"
        }
        vendor {
            name = "James Woolfenden"
            url = "https://github.com/JamesWoolfenden/ghat"
        }
    }
}
