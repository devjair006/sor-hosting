package operations

import "strings"

func publicFailure(err error)(string,string){
 message:=err.Error()
 switch{
 case strings.Contains(message,"unmanaged vhost"),strings.Contains(message,"already registered"),strings.Contains(message,"already adopted"),strings.Contains(message,"already associated"):
  return "RESOURCE_CONFLICT","El dominio o contenedor ya está asociado. Revisa su configuración antes de reintentar."
 case strings.Contains(message,"managed Nginx include"):
  return "PROXY_NOT_CONFIGURED","Completa la integración de SOR Hosting con Nginx en el servidor."
 case strings.Contains(message,"insufficient disk"),strings.Contains(message,"size limit"):
  return "INSUFFICIENT_SPACE","No hay espacio suficiente o el respaldo supera el límite permitido."
 case message=="DNS_PENDING":
  return "DNS_PENDING","Comprueba que todos los registros A y AAAA apunten al servidor."
 case strings.Contains(message,"checksum"),strings.Contains(message,"unsafe archive"),strings.Contains(message,"unsupported backup"):
  return "INVALID_BACKUP","El respaldo no superó la validación. El sitio debe conservar su estado anterior."
 case strings.Contains(message,"safety backup failed"):
  return "SAFETY_BACKUP_FAILED","Falló el respaldo previo. No se inició la restauración."
 case strings.Contains(message,"database readiness"):
  return "DATABASE_NOT_READY","La base de datos no inició a tiempo. Revisa el contenedor y sus logs."
 case strings.Contains(message,"WordPress readiness"):
  return "WORDPRESS_NOT_READY","WordPress no respondió a tiempo. Revisa el contenedor del sitio."
 case strings.Contains(message,"certbot"):
  return "CERTIFICATE_FAILED","No se pudo completar la operación SSL. Comprueba DNS, conectividad y Certbot."
 default:
  return "OPERATION_FAILED","No se pudo completar la operación. Revisa el estado del recurso y los registros del servicio."
 }
}
