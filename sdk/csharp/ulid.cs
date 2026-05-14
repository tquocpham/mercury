using System;
// Note: No extra "using" usually required for the Cysharp library 
// as it lives in the root or 'System' namespace.

public static class IdGenerator
{
    public static string NewOrderID()
    {
        // Generates the ULID
        // It's a struct, so it's very memory efficient
        return Ulid.NewUlid().ToString();
    }
}
