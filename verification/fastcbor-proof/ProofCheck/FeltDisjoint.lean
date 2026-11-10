import ProofCheck.SkipDisjoint
import ProofCheck.FeltDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every nonnil felt array requires an available container depth. -/
theorem feltValid_some_depth (d : Nat) (wire : List w8) (value : GoArray w64 4)
    (valid : FeltValid d wire (some value)) : d < 32 := by
  cases valid <;> assumption

/-- Complete felt success/error separation covers failed initial expectations,
    depth, unsigned limbs and arbitrary recursively skipped extra values. -/
theorem feltValid_not_failure (d offset : Nat) (item rest : List w8) (value : Option (GoArray w64 4))
    (reason : FeltFailureReason) (valid : FeltValid d item value)
    (failure : FeltFailure d (item ++ rest) offset reason) : False := by
  cases value with
  | none =>
    obtain ⟨head, rfl, hn⟩ := feltValid_none d item valid
    have excluded := feltFailure_not_nullish d offset _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | some value =>
    have depth := feltValid_some_depth d item value valid
    obtain ⟨head, payload, argument, items, extras, rfl, enc, major, count, size, children, rfl⟩ :=
      feltValid_some_parts d item value valid
    generalize eqWire : ((head :: payload) ++ unsignedBytes items ++ extras.flatten) ++ rest = wire at failure
    cases failure with
    | outer hn hu failure =>
      exact expectedFailure_excludes_encoded (W8 128) head payload
        (unsignedBytes items ++ extras.flatten ++ rest) _ argument _ enc major
        (by simpa only [List.append_assoc] using eqWire.symm) failure
    | depth tail enc2 major2 => omega
    | @limb d other payload2 tail argument2 done reason enc2 major2 bound short more failure =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (unsignedBytes items ++ extras.flatten ++ rest) (unsignedBytes done ++ tail) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst other; subst argument2
      have moreItems : done.length < items.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix UnsignedWire.bytes done items tail
        (extras.flatten ++ rest) moreItems
        (fun a _ b _ restA restB same => (unsignedWire_prefix_unique a b restA restB same).2.2)
        (by simpa only [unsignedBytes, List.append_assoc] using tailEq.symm)
      subst tail
      exact expectedFailure_excludes_encoded (W8 0) next.head next.payload suffix _ next.value _
        next.argument next.major rfl failure
    | @extras d offset other payload2 tail argument2 done ignored reason enc2 major2 bound four more ignoredValid failure =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (unsignedBytes items ++ extras.flatten ++ rest) (unsignedBytes done ++ ignored.flatten ++ tail)
        argument argument2 enc enc2 (by simpa only [List.append_assoc] using eqWire)
      subst other; subst argument2
      have lengths : done.length = items.length := by omega
      obtain ⟨_, _, extrasEq⟩ := wireSequence_prefix_unique UnsignedWire.bytes UnsignedWire.value
        done items (ignored.flatten ++ tail) (extras.flatten ++ rest) lengths
        (fun a _ b _ => unsignedWire_prefix_unique a b)
        (by simpa only [unsignedBytes, List.append_assoc] using tailEq.symm)
      have moreExtras : ignored.length < extras.length := by omega
      obtain ⟨child, suffix, split, childValid⟩ := skipItems_after_prefix (d + 1) ignored extras tail rest
        ignoredValid children moreExtras extrasEq
      subst tail
      exact skipValid_not_failure (d + 1) offset child suffix reason childValid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_some_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_not_failure
