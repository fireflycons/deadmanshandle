package com.fireflycons.deadmanshandle.push

/**
 * A push from the service, as internal/adapters/sns_push_sender.go sends it:
 * data `{"v":"1","kind":"…","title":"…","body":"…"}`. The title and body are
 * the owner's email's subject and text, or an alarm's.
 */
data class PushMessage(val kind: String, val title: String, val body: String) {
    /** Warnings to check in are reminders; everything else needs attention */
    val channel: Channel
        get() = if (kind == WARNING) Channel.REMINDERS else Channel.ALERTS

    /**
     * The notification's ID. A notice of a kind replaces the last one, but
     * different alarms are kept apart.
     */
    val notificationId: Int
        get() = if (kind == ALARM) "$kind:$title".hashCode() else kind.hashCode()

    enum class Channel(val id: String) {
        ALERTS("alerts"),
        REMINDERS("reminders"),
    }

    companion object {
        private const val WARNING = "warning"
        private const val ALARM = "alarm"

        /** The push in a message's data, or null without a title and body */
        fun parse(data: Map<String, String>): PushMessage? {
            val title = data["title"]
            val body = data["body"]
            if (title.isNullOrBlank() || body == null) return null
            return PushMessage(data["kind"].orEmpty(), title, body)
        }
    }
}
