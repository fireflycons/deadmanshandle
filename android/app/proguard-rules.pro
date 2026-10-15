# Creating the ML Kit code scanner client throws a NullPointerException in
# its internal classes once R8 has processed them, so ML Kit is kept whole
-keep class com.google.mlkit.** { *; }
-keep class com.google.android.gms.internal.mlkit_** { *; }
