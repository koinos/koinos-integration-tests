#!/usr/bin/python3

TOKEN_SUPPLY = 5000000000000000
TOKEN_SUPPLY_PER_BLOCK = TOKEN_SUPPLY // (86400 * 5 * 3 // 3)

# This is the constant for a half life of 3 days.
# It is derived using the following formula: (2 ^ (-1 / num_blocks)) * 2^64
DECAY_CONSTANT = 18446596084619782819

# This is the constant for 1 minus the decay percent.
# It is derivded using the following formula: (1 - (2 ^ (-1 / num_blocks))) * 2^64
ONE_MINUS_DECAY_CONSTANT = 147989089768795

DISK_BUDGET_PER_BLOCK    = 39600
MAX_DISK_PER_BLOCK       = 1 << 19
NETWORK_BUDGET_PER_BLOCK = 1 << 18
MAX_NETWORK_PER_BLOCK    = 1 << 20
COMPUTE_BUDGET_PER_BLOCK = 57500000
MAX_COMPUTE_PER_BLOCK    = 287500000

PRINT_RATE_PREMIUM = 1688
PRINT_RATE_PRECISION = 1000

testConsumption = [
   [106, 562, 513871],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519047],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
   [0, 562, 519541],
]

class ResourceMarket:
   def __init__(self, block_budget, block_limit):
      self.block_budget = block_budget
      self.block_print_rate = int(block_budget * PRINT_RATE_PREMIUM) // PRINT_RATE_PRECISION
      self.resource_supply = int( self.block_print_rate << 64 ) // ONE_MINUS_DECAY_CONSTANT
      self.block_limit = block_limit

   def K(self):
      p = ( ( self.block_print_rate - self.block_budget ) << 64 ) // ONE_MINUS_DECAY_CONSTANT
      return int( ( TOKEN_SUPPLY_PER_BLOCK ) * p // self.block_budget * ( p - self.block_budget ) )

   def overflowTokenLimit(self):
      p = ( ( self.block_print_rate - self.block_budget ) << 64 ) // ONE_MINUS_DECAY_CONSTANT
      uint128Max = ( 1 << 128 ) - 1
      limit = uint128Max // p
      return limit * ( 86400 * 5 * 3 // 3 )

   def calculateLimit(self):
      resource_limit = min(self.resource_supply - 1, self.block_limit)
      k = self.K()

      new_supply = self.resource_supply - resource_limit
      consumed_rc = ( ( k + new_supply - 1 ) // new_supply ) - ( k // self.resource_supply )
      rc_cost = ( consumed_rc + resource_limit - 1 ) // resource_limit

      return int(resource_limit), int(rc_cost)

   def update(self, consumed):
      self.resource_supply = int( self.resource_supply * DECAY_CONSTANT ) >> 64
      self.resource_supply = self.resource_supply + self.block_print_rate - consumed

   def __str__(self):
      return f'resource_supply: {self.resource_supply}, block_budget: {self.block_budget}, block_print_rate: {self.block_print_rate}, block_limit: {self.block_limit}'

class ResourceLimits:
   def __init__(self):
      return

class Markets:
   def __init__(self):
      self.disk_storage = ResourceMarket(DISK_BUDGET_PER_BLOCK, MAX_DISK_PER_BLOCK)
      self.network_bandwidth = ResourceMarket(NETWORK_BUDGET_PER_BLOCK, MAX_NETWORK_PER_BLOCK)
      self.compute_bandwidth = ResourceMarket(COMPUTE_BUDGET_PER_BLOCK, MAX_COMPUTE_PER_BLOCK)

   def getResourceLimits(self):
      limits = ResourceLimits()

      limits.disk_storage_limit,      limits.disk_storage_cost      = self.disk_storage.calculateLimit()
      limits.network_bandwidth_limit, limits.network_bandwidth_cost = self.network_bandwidth.calculateLimit()
      limits.compute_bandwidth_limit, limits.compute_bandwidth_cost = self.compute_bandwidth.calculateLimit()

      return limits

   def consumeBlockResources(self, disk_storage_consumed, network_bandwidth_consumed, compute_bandwidth_consumed):
      self.disk_storage.update(disk_storage_consumed)
      self.network_bandwidth.update(network_bandwidth_consumed)
      self.compute_bandwidth.update(compute_bandwidth_consumed)

def getRcPerBlock(percent):
   rc_available = TOKEN_SUPPLY * percent
   return int(rc_available // (86400 * 5 // 3))

def main():
   rc_per_block = getRcPerBlock(0.25)

   markets = Markets()

   print(f'Initial Conditions:')
   print(f'   Disk market - {markets.disk_storage}, k: {markets.disk_storage.K()}')
   print(f'   Network market - {markets.network_bandwidth}, k: {markets.network_bandwidth.K()}')
   print(f'   Compute market - {markets.compute_bandwidth}, k: {markets.compute_bandwidth.K()}\n')

   print('Spam conditions, spending a max of 25% RC, up to block limit if available')

   for i in range(400000):
      limits = markets.getResourceLimits()

      rc_per_resource = rc_per_block // 3
      disk_used = min(limits.disk_storage_limit, rc_per_resource // limits.disk_storage_cost)
      network_used = min(limits.network_bandwidth_limit, rc_per_resource // limits.network_bandwidth_cost)
      compute_used = min(limits.compute_bandwidth_limit, rc_per_resource // limits.compute_bandwidth_cost)

      markets.consumeBlockResources(disk_used, network_used, compute_used)

      if i % 10000 == 0:
         print(f'Block: {i}, Disk Cost: {limits.disk_storage_cost}, Network Cost: {limits.network_bandwidth_cost}, Compute Cost: {limits.compute_bandwidth_cost}')

   print('\nBack to no usage')

   for i in range(400000):
      limits = markets.getResourceLimits()
      markets.consumeBlockResources(0, 0, 0)

      if i % 10000 == 0:
         print(f'Block: {i+400000}, Disk Cost: {limits.disk_storage_cost}, Network Cost: {limits.network_bandwidth_cost}, Compute Cost: {limits.compute_bandwidth_cost}')

   print('\n')

   for i in range(21):
      rc_per_block = getRcPerBlock(5 * i / 100)

      for j in range(100000):
         limits = markets.getResourceLimits()

         rc_per_resource = rc_per_block // 3
         disk_used = min(limits.disk_storage_limit, rc_per_resource // limits.disk_storage_cost)
         network_used = min(limits.network_bandwidth_limit, rc_per_resource // limits.network_bandwidth_cost)
         compute_used = min(limits.compute_bandwidth_limit, rc_per_resource // limits.compute_bandwidth_cost)

         markets.consumeBlockResources(disk_used, network_used, compute_used)

      limits = markets.getResourceLimits()

      rc_per_resource = rc_per_block // 3
      disk_used = min(limits.disk_storage_limit, rc_per_resource // limits.disk_storage_cost)
      network_used = min(limits.network_bandwidth_limit, rc_per_resource // limits.network_bandwidth_cost)
      compute_used = min(limits.compute_bandwidth_limit, rc_per_resource // limits.compute_bandwidth_cost)

      print(f'---------- RC Used: {5 * i}%')
      print(f'---------- Resources Used: ~{disk_used / DISK_BUDGET_PER_BLOCK * 100}%')
      print(f'Disk Cost: {limits.disk_storage_cost}, Network Cost: {limits.network_bandwidth_cost}, Compute Cost: {limits.compute_bandwidth_cost}')
      print(f'   Disk market - {markets.disk_storage}, k: {markets.disk_storage.K()}')
      print(f'   Network market - {markets.network_bandwidth}, k: {markets.network_bandwidth.K()}')
      print(f'   Compute market - {markets.compute_bandwidth}, k: {markets.compute_bandwidth.K()}')
      print(f'   Comsuming {disk_used} disk, {network_used} network, {compute_used} compute\n')

   markets = Markets()

   print('Integration test values:\n')

   print('testValues := []marketTestValue{')

   for c in testConsumption:
      markets.consumeBlockResources(c[0], c[1], c[2])

      limits = markets.getResourceLimits()
      print('   {' + f'DiskSupply: {markets.disk_storage.resource_supply}, DiskCost: {limits.disk_storage_cost}, NetworkSupply: {markets.network_bandwidth.resource_supply}, NetworkCost: {limits.network_bandwidth_cost}, ComputeSupply: {markets.compute_bandwidth.resource_supply}, ComputeCost: {limits.compute_bandwidth_cost}' + '},')

   print('}\n')

   diskTokenLimit = markets.disk_storage.overflowTokenLimit()
   networkTokenLimit = markets.network_bandwidth.overflowTokenLimit()
   computeTokenLimit = markets.compute_bandwidth.overflowTokenLimit()

   print(f'Token overflow limit: {min(diskTokenLimit, networkTokenLimit, computeTokenLimit)}\n')

   return 0

main()
