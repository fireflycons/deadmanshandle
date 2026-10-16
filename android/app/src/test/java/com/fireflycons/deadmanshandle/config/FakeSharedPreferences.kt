package com.fireflycons.deadmanshandle.config

import android.content.SharedPreferences

/** In-memory SharedPreferences for JVM tests; listeners are not supported */
class FakeSharedPreferences : SharedPreferences {
    private val values = mutableMapOf<String, Any?>()

    override fun getAll(): Map<String, *> = values.toMap()
    override fun getString(key: String, defValue: String?) = values[key] as String? ?: defValue
    override fun getStringSet(key: String, defValues: Set<String>?) =
        @Suppress("UNCHECKED_CAST") (values[key] as Set<String>?) ?: defValues
    override fun getInt(key: String, defValue: Int) = values[key] as Int? ?: defValue
    override fun getLong(key: String, defValue: Long) = values[key] as Long? ?: defValue
    override fun getFloat(key: String, defValue: Float) = values[key] as Float? ?: defValue
    override fun getBoolean(key: String, defValue: Boolean) = values[key] as Boolean? ?: defValue
    override fun contains(key: String) = key in values
    override fun edit(): SharedPreferences.Editor = Editor()

    override fun registerOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener,
    ) = throw UnsupportedOperationException()

    override fun unregisterOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener,
    ) = throw UnsupportedOperationException()

    private inner class Editor : SharedPreferences.Editor {
        private val changes = mutableMapOf<String, Any?>()
        private val removals = mutableSetOf<String>()
        private var clearAll = false

        override fun putString(key: String, value: String?) = apply { changes[key] = value }
        override fun putStringSet(key: String, values: Set<String>?) = apply { changes[key] = values }
        override fun putInt(key: String, value: Int) = apply { changes[key] = value }
        override fun putLong(key: String, value: Long) = apply { changes[key] = value }
        override fun putFloat(key: String, value: Float) = apply { changes[key] = value }
        override fun putBoolean(key: String, value: Boolean) = apply { changes[key] = value }
        override fun remove(key: String) = apply { removals += key }
        override fun clear() = apply { clearAll = true }

        // As in Android: clear first, then removals and puts; a null put removes
        override fun commit(): Boolean {
            if (clearAll) values.clear()
            removals.forEach(values::remove)
            changes.forEach { (key, value) -> if (value == null) values.remove(key) else values[key] = value }
            return true
        }

        override fun apply() {
            commit()
        }
    }
}
