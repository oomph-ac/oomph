// func distanceSqr(a, b mgl32.Vec3) float32
TEXT ·distanceSqr(SB), $0-28
    // Load vector a (12 bytes = 3 floats)
    MOVSS a+0(FP), X0    // X0 = a[0]
    MOVSS a+4(FP), X1    // X1 = a[1]
    MOVSS a+8(FP), X2    // X2 = a[2]
    
    // Load vector b (12 bytes = 3 floats)
    MOVSS b+12(FP), X3   // X3 = b[0]
    MOVSS b+16(FP), X4   // X4 = b[1]
    MOVSS b+20(FP), X5   // X5 = b[2]
    
    // Calculate differences: a - b
    SUBSS X3, X0         // X0 = a[0] - b[0] = dx
    SUBSS X4, X1         // X1 = a[1] - b[1] = dy
    SUBSS X5, X2         // X2 = a[2] - b[2] = dz
    
    // Square the differences
    MULSS X0, X0         // X0 = dx * dx
    MULSS X1, X1         // X1 = dy * dy
    MULSS X2, X2         // X2 = dz * dz
    
    // Sum: dx² + dy² + dz²
    ADDSS X1, X0         // X0 = dx² + dy²
    ADDSS X2, X0         // X0 = dx² + dy² + dz²
    
    // Store result
    MOVSS X0, ret+24(FP)
    RET
