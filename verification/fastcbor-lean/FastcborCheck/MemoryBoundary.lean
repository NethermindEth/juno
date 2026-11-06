import FastcborCheck.GeneratedProof.fastcborprobe.plainunsafe

namespace Perennial
section
variable [GooseExts] [GoLocalContext] [GoGlobalContext] [sem : go.Semantics]

/-- The actual felt layout is a supported byte-backed type, 32 bytes aligned to 8. -/
theorem feltPlainLayout : go.PlainLayout (go.ArrayType 4 go.uint64) 32 8 := by
  exact .array 4 go.uint64 8 8 (.scalar _ _ .isPlainScalar_uint64) (by decide)

instance feltPlain : go.IsPlain (go.ArrayType 4 go.uint64) 32 :=
  ⟨8, feltPlainLayout⟩

/-- This is the semantic rule needed by `cursor.Felt`'s pointer cast.
It proves only that the conversion steps; it does not prove decoding or memory ownership. -/
theorem feltCastSteps (p : Loc) :
    ⟦Convert «unsafe».Pointer (go.PointerType (go.ArrayType 4 go.uint64)), #p⟧
      ⤳[under] #p :=
  go.PlainSemantics.convert_unsafe_to_pointer _ 32 p

end
end Perennial

#print axioms Perennial.feltPlainLayout
#print axioms Perennial.feltCastSteps
