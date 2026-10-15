package com.fireflycons.deadmanshandle.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.fireflycons.deadmanshandle.R

/** The saved config: its endpoint, shown without a prompt, and a way to replace it */
@Composable
fun ConfiguredScreen(endpoint: String, onChange: () -> Unit, modifier: Modifier = Modifier) {
    Column(modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(stringResource(R.string.configured_title), style = MaterialTheme.typography.headlineSmall)
        Text(stringResource(R.string.configured_endpoint, endpoint))
        OutlinedButton(onClick = onChange) { Text(stringResource(R.string.change_config)) }
    }
}
