package com.fireflycons.deadmanshandle.checkin

import com.fireflycons.deadmanshandle.config.CheckInConfig
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** Checks in to the API. [checkIn] blocks, so call it off the main thread. */
class CheckInClient {
    fun checkIn(config: CheckInConfig): CheckInResult {
        val connection = try {
            URL(checkInUrl(config.endpoint)).openConnection() as HttpURLConnection
        } catch (e: IOException) {
            return CheckInResult.Unreachable(e.toString())
        }
        return try {
            connection.requestMethod = "POST"
            connection.setRequestProperty("x-api-key", config.apiKey)
            connection.connectTimeout = TIMEOUT_MILLIS
            connection.readTimeout = TIMEOUT_MILLIS
            // An empty body
            connection.doOutput = true
            connection.setFixedLengthStreamingMode(0)
            connection.outputStream.close()

            val status = connection.responseCode
            val stream = if (status < 400) connection.inputStream else connection.errorStream
            val body = stream?.bufferedReader()?.use { it.readText() }
            CheckInResult.from(status, body)
        } catch (e: IOException) {
            CheckInResult.Unreachable(e.toString())
        } finally {
            connection.disconnect()
        }
    }

    private companion object {
        const val TIMEOUT_MILLIS = 15_000
    }
}
