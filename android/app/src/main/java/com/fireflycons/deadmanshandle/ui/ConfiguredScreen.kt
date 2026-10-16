package com.fireflycons.deadmanshandle.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.fireflycons.deadmanshandle.R

/** What the last check-in attempt came to */
data class CheckInStatus(val text: String, val isError: Boolean)

/**
 * The saved config: its endpoint, shown without a prompt, a check-in button
 * (disabled while [checkingIn]) and a way to replace the config
 */
@Composable
fun ConfiguredScreen(
    endpoint: String,
    status: CheckInStatus?,
    checkingIn: Boolean,
    onCheckIn: () -> Unit,
    onChange: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(stringResource(R.string.configured_title), style = MaterialTheme.typography.headlineSmall)
        Text(stringResource(R.string.configured_endpoint, endpoint))
        if (status != null) {
            Text(
                status.text,
                color = if (status.isError) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = onCheckIn, enabled = !checkingIn) {
                Text(stringResource(if (checkingIn) R.string.checking_in else R.string.check_in))
            }
            OutlinedButton(onClick = onChange, enabled = !checkingIn) {
                Text(stringResource(R.string.change_config))
            }
        }
    }
}
