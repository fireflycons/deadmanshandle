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

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            // Debug-signed until release signing is set up
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    buildFeatures {
        compose = true
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    debugImplementation(libs.androidx.compose.ui.tooling)

    testImplementation(libs.junit)
}
