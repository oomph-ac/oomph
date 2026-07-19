package entity

const (
	DataKeyFlags             = iota
	DataKeyOwnerID           = 5
	DataKeyTargetID          = 6
	DataKeyFireworkMetadata  = 16
	DataKeyScale             = 38
	DataKeyBoundingBoxWidth  = 53
	DataKeyBoundingBoxHeight = 54
)

const (
	DataFlagOnFire = iota
	DataFlagSneaking
	DataFlagRiding
	DataFlagSprinting
	DataFlagAction
	DataFlagInvisible
	DataFlagTempted
	DataFlagInLove
	DataFlagSaddled
	DataFlagPowered
	DataFlagIgnited
	DataFlagBaby
	DataFlagConverting
	DataFlagCritical
	DataFlagCanShowNameTag
	DataFlagAlwaysShowNameTag
	DataFlagImmobile
	DataFlagSilent
	DataFlagWallClimbing
	DataFlagCanClimb
	DataFlagSwimmer
	DataFlagCanFly
	DataFlagWalker
	DataFlagResting
	DataFlagSitting
	DataFlagAngry
	DataFlagInterested
	DataFlagCharged
	DataFlagTamed
	DataFlagOrphaned
	DataFlagLeashed
	DataFlagSheared
	DataFlagGliding
	DataFlagElder
	DataFlagMoving
	DataFlagBreathing
	DataFlagChested
	DataFlagStackable
	DataFlagShowBase
	DataFlagRearing
	DataFlagVibrating
	DataFlagIdling
	DataFlagEvokerSpell
	DataFlagChargeAttack
	DataFlagWASDControlled
	DataFlagCanPowerJump
	DataFlagCanDash
	DataFlagLinger
	DataFlagHasCollision
	DataFlagAffectedByGravity
	DataFlagFireImmune
	DataFlagDancing
	DataFlagEnchanted
	DataFlagShowTridentRope  // tridents show an animated rope when enchanted with loyalty after they are thrown and return to their owner. To be combined with DATA_OWNER_EID
	DataFlagContainerPrivate // inventory is private, doesn't drop contents when killed if true
	DataFlagTransforming
	DataFlagSpinAttack
	DataFlagSwimming
	DataFlagBribed // dolphins have this set when they go to find treasure for the player
	DataFlagPregnant
	DataFlagLayingEgg
	DataFlagRiderCanPick // ???
	DataFlagTransitionSitting
	DataFlagEating
	DataFlagLayingDown
	DataFlagSneezing
	DataFlagTrusting
	DataFlagRolling
	DataFlagScared
	DataFlagInScaffolding
	DataFlagOverScaffolding
	DataFlagFallThroughScaffolding
	DataFlagBlocking // shield
	DataFlagTransitionBlocking
	DataFlagBlockedUsingShield
	DataFlagBlockedUsingDamagedShield
	DataFlagSleeping
	DataFlagWantsToWake
	DataFlagTradeInterest
	DataFlagDoorBreaker // ...
	DataFlagBreakingObstruction
	DataFlagDoorOpener // ...
	DataFlagIllagerCaptain
	DataFlagStunned
	DataFlagRoaring
	DataFlagDelayedAttacking
	DataFlagAvoidingMobs
	DataFlagAvoidingBlock
	DataFlagFacingTargetToRangeAttack
	DataFlagHiddenWhenInvisible // ???????????????????
	DataFlagIsInUI
	DataFlagStalking
	DataFlagEmoting
	DataFlagCelebrating
	DataFlagAdmiring
	DataFlagCelebratingSpecial
	DataFlagOutOfControl
	DataFlagRamAttack
	DataFlagPlayingDead
	DataFlagInAscendableBlock
	DataFlagOverDescendableBlock
	DataFlagCroaking
	DataFlagEatMob
	DataFlagJumpGoalJump
	DataFlagEmerging
	DataFlagSniffing
	DataFlagDigging
	DataFlagSonicBoom
	DataFlagHasDashCooldown
	DataFlagPushTowardsClosestSpace
	DataFlagScenting
	DataFlagRising
	DataFlagHappy
	DataFlagSearching
	DataFlagCrawling
	DataFlagTimerFlag1
	DataFlagTimerFlag2
	DataFlagTimerFlag3
	DataFlagBodyRotationBlocked
	DataFlagRenderWhenInvisible
	DataFlagRotationAxisAligned
	DataFlagCollidable
	DataFlagWASDAirControlled
	DataFlagDoesServerAuthOnlyDismount
	DataFlagBodyRotationAlwaysFollowsHead

	DataFlagNumberOfFlags
)
