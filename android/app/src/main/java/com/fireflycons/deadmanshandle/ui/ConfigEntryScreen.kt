package com.fireflycons.deadmanshandle.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.fireflycons.deadmanshandle.R
import com.fireflycons.deadmanshandle.config.CheckInConfig

/**
 * Entering the endpoint and API key by hand. [onSave] gets a valid config;
 * [onCancel] is null when there is no saved config to go back to.
 */
@Composable
fun ConfigEntryScreen(
    initialEndpoint: String,
    message: String?,
    onSave: (CheckInConfig) -> Unit,
    onCancel: (() -> Unit)?,
    modifier: Modifier = Modifier,
) {
    var endpoint by rememberSaveable { mutableStateOf(initialEndpoint) }
    // Not saveable, so the key stays out of the saved instance state
    var apiKey by remember { mutableStateOf("") }
    var showKey by remember { mutableStateOf(false) }
    // Errors are shown once Save has been pressed
    var checked by rememberSaveable { mutableStateOf(false) }

    val endpointValid = CheckInConfig.isValidEndpoint(endpoint.trim())
    val apiKeyValid = CheckInConfig.isValidApiKey(apiKey)

    Column(
        modifier.verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(stringResource(R.string.config_title), style = MaterialTheme.typography.headlineSmall)
        Text(stringResource(R.string.config_explanation))
        if (message != null) {
            Text(message, color = MaterialTheme.colorScheme.error)
        }
        OutlinedTextField(
            value = endpoint,
            onValueChange = { endpoint = it },
            label = { Text(stringResource(R.string.endpoint_label)) },
            singleLine = true,
            isError = checked && !endpointValid,
            supportingText = if (checked && !endpointValid) {
                { Text(stringResource(R.string.endpoint_invalid)) }
            } else null,
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Uri,
                autoCorrectEnabled = false,
                imeAction = ImeAction.Next,
            ),
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = apiKey,
            onValueChange = { apiKey = it },
            label = { Text(stringResource(R.string.api_key_label)) },
            singleLine = true,
            isError = checked && !apiKeyValid,
            supportingText = if (checked && !apiKeyValid) {
                { Text(stringResource(R.string.api_key_invalid)) }
            } else null,
            visualTransformation = if (showKey) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                TextButton(onClick = { showKey = !showKey }) {
                    Text(stringResource(if (showKey) R.string.hide else R.string.show))
                }
            },
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Password,
                autoCorrectEnabled = false,
                imeAction = ImeAction.Done,
            ),
            modifier = Modifier.fillMaxWidth(),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = {
                checked = true
                if (endpointValid && apiKeyValid) onSave(CheckInConfig(endpoint.trim(), apiKey))
            }) {
                Text(stringResource(R.string.save))
            }
            if (onCancel != null) {
                OutlinedButton(onClick = onCancel) { Text(stringResource(R.string.cancel)) }
            }
        }
    }
}
