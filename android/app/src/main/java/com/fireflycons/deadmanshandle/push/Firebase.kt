package com.fireflycons.deadmanshandle.push

import android.content.Context
import com.fireflycons.deadmanshandle.config.PushOptions
import com.google.android.gms.tasks.Tasks
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

/**
 * Firebase, started at run time from the config's [PushOptions] rather than
 * from a google-services.json built into the app, since each deployment has
 * its own Firebase project.
 *
 * Firebase Messaging 26 deprecates the token API for register() and the
 * service's onRegistered, which an app must opt in to with a manifest flag.
 * The token API is kept until SNS is known to accept tokens from the new one.
 */
@Suppress("DEPRECATION")
object Firebase {
    /**
     * Starts Firebase with [options], or stops it if null. Changing or
     * removing the options first deletes this install's token, so the old
     * project can no longer push to it.
     */
    fun configure(context: Context, options: PushOptions?) {
        val wanted = options?.let(::firebaseOptions)
        val current = defaultApp(context)
        when {
            current == null -> if (wanted != null) FirebaseApp.initializeApp(context, wanted)
            current.options == wanted -> {}
            // The listener runs on the main thread
            else -> FirebaseMessaging.getInstance().deleteToken().addOnCompleteListener {
                current.delete()
                if (wanted != null) FirebaseApp.initializeApp(context, wanted)
            }
        }
    }

    /**
     * This install's push token, or null if push is off or the token can't be
     * had. It blocks, so call it off the main thread.
     */
    fun token(context: Context): String? {
        if (defaultApp(context) == null) return null
        return try {
            Tasks.await(FirebaseMessaging.getInstance().token, TOKEN_TIMEOUT_SECONDS, TimeUnit.SECONDS)
        } catch (_: ExecutionException) {
            null
        } catch (_: TimeoutException) {
            null
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            null
        }
    }

    private fun defaultApp(context: Context): FirebaseApp? =
        FirebaseApp.getApps(context).firstOrNull { it.name == FirebaseApp.DEFAULT_APP_NAME }

    private fun firebaseOptions(options: PushOptions) = FirebaseOptions.Builder()
        .setProjectId(options.projectId)
        .setApplicationId(options.appId)
        .setApiKey(options.apiKey)
        .setGcmSenderId(options.senderId)
        .build()

    private const val TOKEN_TIMEOUT_SECONDS = 10L
}
