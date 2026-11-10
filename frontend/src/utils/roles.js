// Roles del sistema. Los permisos los controlan el gateway y los servicios; acá solo decidimos qué mostrar.
export const DIRECTOR = "DIRECTOR";
export const ADMIN_SUCURSAL = "ADMIN_SUCURSAL";
export const VENDEDOR = "VENDEDOR";
export const ALL_ROLES = [DIRECTOR, ADMIN_SUCURSAL, VENDEDOR];

export const ROLE_LABEL = {
  DIRECTOR: "Director",
  ADMIN_SUCURSAL: "Admin de sucursal",
  VENDEDOR: "Vendedor",
};

export function isDirector(user) {
  return user?.role === DIRECTOR;
}

// Ajustan stock el director en cualquier sucursal y el admin solo en la suya.
export function canAdjust(user, branchId) {
  return isDirector(user) || (user?.role === ADMIN_SUCURSAL && user.branchId === Number(branchId));
}

// Crean usuarios el director (cualquier rol) y el admin (vendedores de su sucursal).
export function canCreateUsers(user) {
  return user?.role === DIRECTOR || user?.role === ADMIN_SUCURSAL;
}
