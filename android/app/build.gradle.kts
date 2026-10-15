plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

// The app is versioned with the rest of the project, from VERSION (semver)
val appVersion = rootProject.file("../VERSION").readText().trim()
val (major, minor, patch) = appVersion.split(".").map { it.toInt() }

android {
    namespace = "com.fireflycons.deadmanshandle"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.fireflycons.deadmanshandle"
        minSdk = 30 // auth-bound Keystore keys accepting biometric or device credential
        targetSdk = 37
        versionCode = major * 10000 + minor * 100 + patch
        versionName = appVersion
    }

    // The release key, from the environment (the release workflow sets it from
    // repository secrets). The keystore is PKCS12, so the key's password is
    // the store's.
    val releaseKeystore = System.getenv("ANDROID_KEYSTORE_FILE")
    signingConfigs {
        if (releaseKeystore != null) {
            create("release") {
                storeFile = file(releaseKeystore)
                storePassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("ANDROID_KEY_ALIAS")
                keyPassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            // Debug-signed without the release key, as in local builds and CI
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    buildFeatures {
        compose = true
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.biometric)
    implementation(libs.androidx.fragment)
    implementation(libs.play.services.code.scanner)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    debugImplementation(libs.androidx.compose.ui.tooling)

    testImplementation(libs.junit)
    testImplementation(libs.org.json)
}
