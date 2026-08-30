# MSG91 Android OTP setup

Each Android app reads its MSG91 widget credentials from its own ignored
`local.properties` file, Gradle user properties, or environment variables.
The order of precedence is Gradle property, environment variable, then local
property.

```properties
MSG91_WIDGET_ID=your-widget-id
MSG91_AUTH_TOKEN=your-mobile-integration-token
```

Create a separate MSG91 widget/token for the listener app unless the MSG91
widget has been explicitly configured to allow both Android application IDs.
The backend needs only this server-side variable:

```dotenv
MSG91_SERVER_AUTH_KEY=your-server-side-msg91-auth-key
```

The Android SDK sends and retries OTPs. Once the user enters their code, the
app calls `OTPWidget.verifyOTP` and sends MSG91's returned access token to
TrueLine. The backend validates that token with MSG91's
`verifyAccessToken` API before issuing a JWT. The server auth key belongs only
in the backend environment; do not add it to either Android app or commit it.

TrueLine requires the verified MSG91 identifier (from the verification response
or its JWT) to match the phone number being logged in. This prevents a valid
token for one number from being used to create a session for another number.

The current TrueLine screens use a six-digit code-entry flow. Leave MSG91
**invisible verification** disabled in each widget until an automatic sign-in
screen is added for the no-code result.

In the MSG91 widget configuration, enable **Mobile Integration** and register
the correct Android package/signing details for each app. The customer app is
`com.example.truelineapp`; the listener app is
`com.example.trueline_listener`.
