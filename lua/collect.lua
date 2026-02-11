local f=game.forces["player"] local e=game.forces["enemy"] local s=game.surfaces[1] local r={}
r.tick=game.tick
r.players=#game.connected_players
r.evolution=e.get_evolution_factor(s)
local ip=f.get_item_production_statistics(s)
r.item_production=ip.input_counts
r.item_consumption=ip.output_counts
local fp=f.get_fluid_production_statistics(s)
r.fluid_production=fp.input_counts
r.fluid_consumption=fp.output_counts
local kc=f.get_kill_count_statistics(s)
r.kill_counts=kc.input_counts
local eb=f.get_entity_build_count_statistics(s)
r.entity_built=eb.input_counts
r.rockets_launched=f.rockets_launched
r.research=f.current_research and f.current_research.name or nil
r.research_progress=f.research_progress
local poles=s.find_entities_filtered{type="electric-pole",limit=1}
if poles[1] then
  local en=poles[1].electric_network_statistics
  r.power_production=en.input_counts
  r.power_consumption=en.output_counts
end
rcon.print(helpers.table_to_json(r))
