// func distanceSqr(a, b mgl32.Vec3) float32
TEXT ·distanceSqr(SB), $0-28
    // Load vector a (12 bytes = 3 floats)
    MOVW a+0(FP), R0     // Load address of a
    FMOVS 0(R0), F0      // F0 = a[0]
    FMOVS 4(R0), F1      // F1 = a[1]
    FMOVS 8(R0), F2      // F2 = a[2]
    
    // Load vector b (12 bytes = 3 floats)
    MOVW b+12(FP), R1    // Load address of b
    FMOVS 0(R1), F3      // F3 = b[0]
    FMOVS 4(R1), F4      // F4 = b[1]
    FMOVS 8(R1), F5      // F5 = b[2]
    
    // Calculate differences: a - b
    FSUBS F3, F0, F0     // F0 = a[0] - b[0] = dx
    FSUBS F4, F1, F1     // F1 = a[1] - b[1] = dy
    FSUBS F5, F2, F2     // F2 = a[2] - b[2] = dz
    
    // Square the differences
    FMULS F0, F0, F0     // F0 = dx * dx
    FMULS F1, F1, F1     // F1 = dy * dy
    FMULS F2, F2, F2     // F2 = dz * dz
    
    // Sum: dx² + dy² + dz²
    FADDS F1, F0, F0     // F0 = dx² + dy²
    FADDS F2, F0, F0     // F0 = dx² + dy² + dz²
    
    // Store result
    FMOVS F0, ret+24(FP)
    RET
